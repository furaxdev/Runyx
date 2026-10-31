// Command runyx is the Runyx CLI, daemon, and HTTP server, all in one
// binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	// Reattaches stdio to a launching terminal's console on Windows, a
	// no-op everywhere else. Must run before anything touches
	// stdout/stderr -- see console_windows.go for why.
	attachParentConsole()

	if len(os.Args) < 2 {
		// Bare `runyx` (double-clicked, or run with no arguments from a
		// terminal) opens the app window -- the whole point of this
		// being "a real Windows application, not a site in a browser"
		// is that this is what happens with zero flags to remember.
		// `runyx tui` is still how to get the terminal UI explicitly.
		exitOnError(cmdGUI(nil))
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error

	switch cmd {
	case "version":
		err = cmdVersion(args)
	case "serve":
		err = cmdServe(args)
	case "gui":
		err = cmdGUI(args)
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
  runyx                      Open the app window (same as 'runyx gui')
  runyx gui                  Open the app window, starting the daemon if needed
  runyx tui                  Launch the terminal UI (needs runyx-tui alongside this binary)
  runyx serve                Run the daemon in the foreground, no window
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
