package main

import (
	"fmt"
	"os"

	"github.com/dan-slater/cc-lens/cmd"
)

const usage = `cc-lens — zero-dep HTTP/SSE lens over running Claude Code sessions

Usage:
  cc-lens start [flags]            Run the HTTP server
  cc-lens install-hooks [flags]    Patch ~/.claude/settings.json
  cc-lens tmux-relay [flags]       Deliver queued messages to tmux panes

Run "cc-lens <command> -h" for command-specific flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "start":
		err = cmd.Start(os.Args[2:])
	case "install-hooks":
		err = cmd.InstallHooks(os.Args[2:])
	case "tmux-relay":
		err = cmd.TmuxRelay(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
