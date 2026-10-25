// Command runyx is the Runyx CLI, daemon, and HTTP server, all in one
// binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		// Bare `runyx` on a real terminal launches the TUI, the same way
		// `git` with no args prints help but an interactive shell tool
		// would rather hand you something to look at. Piped/redirected
		// stdout (scripts, CI) keeps the old behavior: usage + exit 1.
		if isStdoutTTY() {
			exitOnError(cmdTUI(nil))
			return
		}
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error

	switch cmd {
	case "version":
		err = cmdVersion(args)
	case "serve":
		err = cmdServe(args)
	case "tui":
		err = cmdTUI(args)
	case "doctor":
		err = cmdDoctor(args)
	case "recommend":
		err = cmdRecommend(args)
	case "tune":
		err = cmdTune(args)
	case "pull":
		err = cmdPull(args)
	case "run":
		err = cmdRun(args)
	case "list", "ls":
		err = cmdList(args)
	case "rm":
		err = cmdRemove(args)
	case "show":
		err = cmdShow(args)
	case "mcp-serve":
		err = cmdMCPServe(args)
	case "bench":
		err = cmdBench(args)
	case "update":
		err = cmdUpdate(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "runyx: unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	exitOnError(err)
}

// exitOnError exits 1 after printing err, unless err is an
// ExitCodeError (from execing a sibling binary like runyx-tui), in
// which case it propagates that exact exit code. A nil err is a no-op.
func exitOnError(err error) {
	if err == nil {
		return
	}
	if exitErr, ok := err.(ExitCodeError); ok {
		os.Exit(exitErr.Code)
	}
	fmt.Fprintf(os.Stderr, "runyx: %v\n", err)
	os.Exit(1)
}

func printUsage() {
	fmt.Fprint(os.Stderr, `runyx — a local LLM runner

Usage:
  runyx version              Print version info
  runyx                      Launch the TUI (if on a real terminal; same as 'runyx tui')
  runyx tui                  Launch the terminal UI
  runyx serve                Run the daemon in the foreground
  runyx doctor                Detect CPU/RAM/GPU and backend availability
  runyx recommend              Suggest models that fit this machine
  runyx tune <model>           Compute and cache tuning params for a model
  runyx pull <repo>[:file]      Download a GGUF model from Hugging Face
  runyx run <model>            Chat with a model (pulling if needed)
  runyx run <model> --selftest  Ask one answer-checkable question and report pass/fail, no chat loop
  runyx list                  List installed models
  runyx show <model>           Show details about an installed model
  runyx rm <model>             Remove an installed model
  runyx mcp-serve              Expose Runyx's tools over MCP (stdio)
  runyx bench <model> [prompt]  Measure tokens/sec and time-to-first-token
  runyx update                 Check for (and, once signed, apply) updates
`)
}
