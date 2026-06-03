// Package cmd implements the agentsmith subcommands (plan sections 2 & 7) for
// phases 1-3: get, list, remove, install, uninstall, status, update, doctor.
package cmd

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jperocho/agentsmith/internal/config"
	"github.com/jperocho/agentsmith/internal/hub"
	"github.com/jperocho/agentsmith/internal/lock"
	"github.com/jperocho/agentsmith/internal/manifest"
)

// parseInterleaved parses fs while tolerating positional arguments appearing
// before, between, or after flags (Go's flag package otherwise stops at the
// first non-flag token). Returns the collected positional arguments.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
	}
}

// App carries shared state across command handlers.
type App struct {
	Hub *hub.Hub
	Cfg *config.Config
}

// New bootstraps the hub and returns an App.
func New() (*App, error) {
	h, err := hub.Default()
	if err != nil {
		return nil, err
	}
	if err := h.Bootstrap(); err != nil {
		return nil, err
	}
	cfg, err := config.Load(h.ConfigPath())
	if err != nil {
		return nil, err
	}
	return &App{Hub: h, Cfg: cfg}, nil
}

// interactive reports whether stdin is a terminal (so prompting is meaningful).
func interactive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// confirm prompts the user with question and returns true on a yes answer.
// When auto is true or stdin is non-interactive it returns true without asking.
func confirm(question string, auto bool) bool {
	if auto || !interactive() {
		return true
	}
	fmt.Printf("%s [y/N]: ", question)
	var ans string
	fmt.Scanln(&ans)
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

// withLockedManifest acquires the hub lock, loads the manifest, runs fn, and on
// success (and only if fn reports it mutated state) saves atomically. The lock
// guards the whole read-modify-write cycle (plan section 4).
func (a *App) withLockedManifest(fn func(m *manifest.Manifest) (changed bool, err error)) error {
	lk, err := lock.Acquire(a.Hub.LockPath())
	if err != nil {
		return err
	}
	defer lk.Unlock()

	m, err := manifest.Load(a.Hub.ManifestPath())
	if err != nil {
		return err
	}
	changed, err := fn(m)
	if err != nil {
		return err
	}
	if changed {
		return m.Save()
	}
	return nil
}

// repoSpec is a parsed `get` argument.
type repoSpec struct {
	URL  string // canonical clone URL
	Ref  string // requested ref (tag/branch), empty = default branch
	Name string // derived skill name
}

// ownerNameRe matches owner/name where each segment starts with an
// alphanumeric/underscore — rejecting "." and ".." path-traversal segments.
var ownerNameRe = regexp.MustCompile(`^\w[\w.-]*/\w[\w.-]*$`)

// parseRepo canonicalizes a repo argument into a clone URL, optional ref, and
// skill name (plan 7.get step 1). Accepts:
//   - owner/name            -> https://github.com/owner/name
//   - https://host/owner/name[.git]
//   - git@host:owner/name.git
//
// A trailing @ref selects a tag/branch: owner/name@v1.2.0.
func parseRepo(arg string) (repoSpec, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return repoSpec{}, fmt.Errorf("empty repo argument")
	}

	ref := ""
	// Split a trailing @ref, but not the @ in a git@host: SSH URL.
	if i := strings.LastIndex(arg, "@"); i > 0 && !strings.HasPrefix(arg, "git@") {
		ref = arg[i+1:]
		arg = arg[:i]
	} else if strings.HasPrefix(arg, "git@") {
		// git@host:owner/name@ref -> last @ after the colon is the ref.
		if colon := strings.Index(arg, ":"); colon >= 0 {
			if j := strings.LastIndex(arg[colon:], "@"); j > 0 {
				ref = arg[colon+j+1:]
				arg = arg[:colon+j]
			}
		}
	}

	var url string
	switch {
	case ownerNameRe.MatchString(arg):
		url = "https://github.com/" + arg
	case strings.HasPrefix(arg, "https://"), strings.HasPrefix(arg, "http://"):
		url = arg
	case strings.HasPrefix(arg, "git@"):
		url = arg
	case strings.HasPrefix(arg, "file://"):
		url = arg
	default:
		return repoSpec{}, fmt.Errorf("unrecognized repo form %q (use owner/name, https URL, or git@ URL)", arg)
	}

	name := deriveName(url)
	if name == "" {
		return repoSpec{}, fmt.Errorf("cannot derive skill name from %q", arg)
	}
	return repoSpec{URL: url, Ref: ref, Name: name}, nil
}

// deriveName extracts the skill name (last path segment, sans .git) from a URL.
func deriveName(url string) string {
	s := url
	s = strings.TrimSuffix(s, ".git")
	// Normalize git@host:owner/name and https://host/owner/name alike.
	s = strings.ReplaceAll(s, ":", "/")
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	// Guard against odd characters in the derived directory name.
	if !regexp.MustCompile(`^[\w.-]+$`).MatchString(last) {
		return ""
	}
	return last
}
