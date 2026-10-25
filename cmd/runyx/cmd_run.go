package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/downloader"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// cmdRun pulls the model if needed, loads it via an in-process backend
// manager, and drops into an interactive chat loop against its
// llama-server instance. "runyx run <model> --selftest" skips the
// interactive loop and instead asks a fixed, answer-checkable question
// once, so a broken model/build pairing (garbled multilingual output,
// not a crash -- see docs/DECISIONS.md's AMD-EPYC demo gibberish entry)
// is caught before anyone assumes "it started, so it must be working."
func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: runyx run <model> [--selftest]")
	}
	name := args[0]
	selftest := false
	for _, a := range args[1:] {
		if a == "--selftest" {
			selftest = true
		}
	}

	cfg, store, err := loadStore()
	if err != nil {
		return err
	}

	if _, ok, _ := store.Get(name); !ok {
		fmt.Printf("Model %q not installed yet, pulling...\n", name)
		if err := cmdPull([]string{name}); err != nil {
			return fmt.Errorf("auto-pull %q: %w", name, err)
		}
	}

	binPath, err := resolveBackendBinaryBlocking(cfg, func(p downloader.Progress) {
		printProgress(p)
	})
	if err != nil {
		fmt.Println()
		return fmt.Errorf("fetch llama-server backend: %w", err)
	}
	fmt.Println()

	mdl, ok, err := store.Get(name)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model %q not found after pull", name)
	}

	if mdl.Tuning == (modelstore.TuningParams{}) {
		info := hwdetect.Detect()
		t := hwdetect.Tune(info, int(mdl.SizeBytes/(1024*1024)))
		mdl.Tuning = modelstore.TuningParams{
			NGPULayers: t.NGPULayers, Threads: t.Threads,
			CtxSize: t.CtxSize, BatchSize: t.BatchSize, FlashAttn: t.FlashAttn,
		}
		_ = store.Put(mdl)
	}

	mgr := backend.NewManager(binPath, 0, cfg.VRAMBudgetMB) // no idle eviction mid-chat
	defer mgr.StopAll()

	fmt.Printf("Loading %s...\n", mdl.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inst, err := mgr.Start(ctx, mdl, mdl.Tuning)
	if err != nil {
		return fmt.Errorf("start model: %w", err)
	}
	if selftest {
		return selfTest(inst.BaseURL, mdl.Name)
	}

	fmt.Printf("Ready. Chatting with %s. Type /exit to quit.\n\n", mdl.Name)

	return chatLoop(inst.BaseURL, mdl.Name)
}

// selfTestPrompt and selfTestWords: a question whose correct answer is
// known ahead of time and narrow enough to check for by substring,
// unlike an open-ended "explain what you are" demo prompt. temperature
// 0 so the same build+model always produces the same tokens, making a
// flake-vs-genuine-break distinction possible across repeated runs.
const selfTestPrompt = "What is 2+2? Answer with one word."

var selfTestWords = []string{"4", "four"}

// selfTest sends selfTestPrompt once and reports whether the reply
// contains an expected answer, never dropping into the interactive
// loop. Returns a non-nil error on anything that would otherwise look
// like a silent success to a script checking only the exit code: no
// reply, a request error, or an answer that doesn't contain "4"/"four".
func selfTest(baseURL, modelName string) error {
	fmt.Printf("Self-test: asking %q (temperature 0, max_tokens 16)...\n", selfTestPrompt)
	client := &http.Client{Timeout: chatRequestTimeout}
	body, _ := json.Marshal(map[string]any{
		"model":       modelName,
		"messages":    []chatMsg{{Role: "user", Content: selfTestPrompt}},
		"stream":      false,
		"max_tokens":  16,
		"temperature": 0,
	})
	resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("self-test request failed: %w", err)
	}
	defer resp.Body.Close()

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("self-test: decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return fmt.Errorf("self-test: no choices in response")
	}
	reply := parsed.Choices[0].Message.Content
	fmt.Printf("Reply: %q\n", reply)

	if selfTestReplyOK(reply) {
		fmt.Println("Self-test PASSED: reply contains an expected answer.")
		return nil
	}
	return fmt.Errorf("self-test FAILED: reply %q does not contain %v -- the model/build pairing is producing unreadable or wrong output", reply, selfTestWords)
}

// selfTestReplyOK reports whether reply contains one of selfTestWords,
// case-insensitively. Split out from selfTest so the matching logic
// (the part actually worth unit-testing) doesn't require a running
// backend.
func selfTestReplyOK(reply string) bool {
	lower := strings.ToLower(reply)
	for _, want := range selfTestWords {
		if strings.Contains(lower, want) {
			return true
		}
	}
	return false
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequestTimeout bounds one full chat exchange (request plus
// reading the entire streamed reply). Without it a runaway generation
// — the model never producing a stop token, which does happen in
// practice, not just hypothetically — hangs this CLI forever with no
// way to recover short of killing the process.
const chatRequestTimeout = 3 * time.Minute

// chatMaxTokens caps how many tokens a single reply may generate, as a
// second line of defense alongside chatRequestTimeout: it turns a
// still-slow-but-not-yet-timed-out runaway generation into a bounded,
// visibly truncated reply instead of 3 minutes of churn.
const chatMaxTokens = 1024

func chatLoop(baseURL, modelName string) error {
	var history []chatMsg
	scanner := bufio.NewScanner(os.Stdin)
	client := &http.Client{Timeout: chatRequestTimeout}

	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "/quit" {
			return nil
		}
		history = append(history, chatMsg{Role: "user", Content: line})

		body, _ := json.Marshal(map[string]any{
			"model":      modelName,
			"messages":   history,
			"stream":     true,
			"max_tokens": chatMaxTokens,
		})
		resp, err := client.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("chat request (no reply within %s — the model may be stuck generating): %w", chatRequestTimeout, err)
		}

		fmt.Print("runyx> ")
		var reply strings.Builder
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			l := sc.Text()
			if !strings.HasPrefix(l, "data: ") {
				continue
			}
			data := strings.TrimPrefix(l, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
				fmt.Print(chunk.Choices[0].Delta.Content)
				reply.WriteString(chunk.Choices[0].Delta.Content)
			}
		}
		resp.Body.Close()
		fmt.Println()
		fmt.Println()
		history = append(history, chatMsg{Role: "assistant", Content: reply.String()})
	}
}
