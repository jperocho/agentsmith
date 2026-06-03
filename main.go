// Command agentsmith is a local "skills hub": it clones skill repos into
// ~/.agentsmith, tracks their versions, and installs them into AI coding agents
// (Claude Code, …) by copy or symlink. See README.md for usage.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/jperocho/agentsmith/internal/cmd"
)

// Build metadata. version is the release tag; it is overridden at build time via
//
//	go build -ldflags "-X main.version=v1.2.3 -X main.commit=abc -X main.date=..."
//
// and otherwise falls back to module/VCS info embedded by the Go toolchain.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

// versionString renders the build version, enriching it from debug.BuildInfo
// when ldflags were not supplied (e.g. `go install` / `go build` without -X).
func versionString() string {
	v, c, d := version, commit, date
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
			case "vcs.time":
				if d == "" {
					d = s.Value
				}
			}
		}
	}
	out := "agentsmith " + v
	if c != "" {
		if len(c) > 10 {
			c = c[:10]
		}
		out += " (" + c
		if d != "" {
			out += ", " + d
		}
		out += ")"
	}
	return out
}

const usage = `agentsmith — local skills hub

Usage:
  agentsmith get <repo>[@ref] [--allow-scripts]   clone a skill into the hub (no install)
  agentsmith list                                  list hub skills + versions + installs
  agentsmith install <skill> [flags]               install a skill into agent(s)
  agentsmith update <skill> [flags]                pull latest, re-resolve, re-sync copies
  agentsmith status <skill>                        show install state + drift
  agentsmith uninstall <skill> [flags]             remove an install from agent(s)
  agentsmith remove <skill> [--keep-installs]      remove a skill from the hub entirely
  agentsmith doctor                                detect agents, validate hub integrity
  agentsmith version                               print build version

Install flags:
  --global               install to user-wide agent config (default: project)
  --mode copy|symlink    install mode (default: config.json defaultMode, else auto)
  --agent <name>         target agent(s); repeatable or comma-separated
  --dry-run              print resolved target(s) + mode; write nothing
  --yes                  non-interactive; assume defaults

Update flags:
  --ref <tag|branch>     re-point the skill at a new ref
  --allow-scripts        keep executable bits in the hub clone
  --yes                  skip the copy-resync confirmation

Repo forms: owner/name | https://host/owner/name | git@host:owner/name.git | file://path
Hub root:   ~/.agentsmith  (override with AGENTSMITH_HOME)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	case "-v", "--version", "version":
		fmt.Println(versionString())
		return
	}

	app, err := cmd.New()
	if err != nil {
		fail(err)
	}

	var runErr error
	switch sub {
	case "get":
		runErr = app.Get(args)
	case "list", "ls":
		runErr = app.List(args)
	case "install":
		runErr = app.Install(args)
	case "update":
		runErr = app.Update(args)
	case "status":
		runErr = app.Status(args)
	case "uninstall":
		runErr = app.Uninstall(args)
	case "remove", "rm":
		runErr = app.Remove(args)
	case "doctor":
		runErr = app.Doctor(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", sub)
		fmt.Print(usage)
		os.Exit(2)
	}
	if runErr != nil {
		fail(runErr)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "agentsmith: %v\n", err)
	os.Exit(1)
}
