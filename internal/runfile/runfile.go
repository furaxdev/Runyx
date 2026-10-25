// Package runfile parses Runyx's "Runfile" format: a shareable YAML
// definition of a model + persona (system prompt, parameters, chat
// template, LoRA adapters), and imports Ollama Modelfiles into the same
// shape. Implements FEATURES.md item 8.
package runfile

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Runfile is a shareable model+persona definition.
type Runfile struct {
	Model        string            `yaml:"model"`                   // base model name (as registered in `runyx list`) or a HF repo to pull
	Name         string            `yaml:"name"`                    // display name for the resulting persona
	SystemPrompt string            `yaml:"system_prompt,omitempty"` // empty = model's own default
	Template     string            `yaml:"template,omitempty"`      // chat template override, empty = model's own
	Parameters   map[string]any    `yaml:"parameters,omitempty"`    // num_ctx, temperature, etc.
	Adapters     []string          `yaml:"adapters,omitempty"`      // paths to LoRA adapter GGUFs
	Metadata     map[string]string `yaml:"metadata,omitempty"`
}

// Load parses a Runfile from path.
func Load(path string) (Runfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Runfile{}, fmt.Errorf("read runfile %s: %w", path, err)
	}
	var rf Runfile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return Runfile{}, fmt.Errorf("parse runfile %s: %w", path, err)
	}
	if rf.Model == "" {
		return Runfile{}, fmt.Errorf("runfile %s: missing required field 'model'", path)
	}
	return rf, nil
}

// Save writes a Runfile as YAML to path.
func Save(path string, rf Runfile) error {
	data, err := yaml.Marshal(rf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ImportOllamaModelfile parses an Ollama Modelfile (documented at
// https://github.com/ollama/ollama/blob/main/docs/modelfile.md) into a
// Runfile. Only the commonly used directives are supported: FROM,
// SYSTEM, TEMPLATE, PARAMETER, ADAPTER. Implemented from the public
// documentation only — no Ollama source was read.
func ImportOllamaModelfile(path string) (Runfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return Runfile{}, fmt.Errorf("open modelfile %s: %w", path, err)
	}
	defer f.Close()

	rf := Runfile{Parameters: map[string]any{}}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		directive, rest := strings.ToUpper(parts[0]), strings.TrimSpace(parts[1])
		rest = unquote(rest)
		switch directive {
		case "FROM":
			rf.Model = rest
		case "SYSTEM":
			rf.SystemPrompt = rest
		case "TEMPLATE":
			rf.Template = rest
		case "ADAPTER":
			rf.Adapters = append(rf.Adapters, rest)
		case "PARAMETER":
			kv := strings.SplitN(rest, " ", 2)
			if len(kv) == 2 {
				rf.Parameters[kv[0]] = parseParamValue(kv[1])
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Runfile{}, err
	}
	if rf.Model == "" {
		return Runfile{}, fmt.Errorf("modelfile %s has no FROM directive", path)
	}
	return rf, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`) && len(s) >= 6 {
		return strings.TrimSpace(s[3 : len(s)-3])
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func parseParamValue(s string) any {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}
