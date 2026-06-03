// Package agent defines the AI-coding-agent adapters: how to detect an agent
// locally and where it expects skills installed (plan section 6). Adding an
// agent = one new Adapter entry in the registry.
package agent

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Scope selects a user-wide vs project-local install target.
type Scope string

const (
	Global  Scope = "global"
	Project Scope = "project"
)

// Adapter describes one supported agent.
type Adapter struct {
	// Name is the manifest/CLI identifier, e.g. "claude-code".
	Name string
	// Title is the human-facing label, e.g. "Claude Code".
	Title string

	detect     func() bool
	globalDir  func() (string, error)
	projectDir func(cwd string) string
}

// Detect reports whether the agent appears installed on this machine.
func (a Adapter) Detect() bool { return a.detect() }

// Dir resolves the install directory for skill in the given scope. cwd is the
// project root for project scope (ignored for global).
func (a Adapter) Dir(scope Scope, cwd, skill string) (string, error) {
	switch scope {
	case Global:
		base, err := a.globalDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, skill), nil
	default:
		return filepath.Join(a.projectDir(cwd), skill), nil
	}
}

// AllowedRoots returns the base directories this adapter may write into, for
// both scopes, used to build the install-target allowlist (security 5.4).
func (a Adapter) AllowedRoots(cwd string) []string {
	var roots []string
	if g, err := a.globalDir(); err == nil {
		roots = append(roots, g)
	}
	roots = append(roots, a.projectDir(cwd))
	return roots
}

func homeJoin(parts ...string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}

func onPATH(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// dotDirAdapter builds an adapter following the common convention of a
// per-agent dot-directory holding a skills/ subdir, both global (~/.x/skills)
// and project (./.x/skills). cliBin and dotDir drive detection.
func dotDirAdapter(name, title, cliBin, dotDir string) Adapter {
	return Adapter{
		Name:  name,
		Title: title,
		detect: func() bool {
			if onPATH(cliBin) {
				return true
			}
			if d, err := homeJoin(dotDir); err == nil {
				return dirExists(d)
			}
			return false
		},
		globalDir:  func() (string, error) { return homeJoin(dotDir, "skills") },
		projectDir: func(cwd string) string { return filepath.Join(cwd, dotDir, "skills") },
	}
}

// registry holds all supported adapters.
//
// NOTE: Claude Code paths are confirmed. The Cursor/Codex/Hermes paths below
// follow the common ~/.x/skills convention but are NOT yet verified against
// each agent's real layout (e.g. Cursor uses .cursor/rules/*.mdc). Confirm with
// `agentsmith install <skill> --agent <name> --dry-run` before relying on them.
var registry = []Adapter{
	dotDirAdapter("claude-code", "Claude Code", "claude", ".claude"),
	dotDirAdapter("cursor", "Cursor", "cursor", ".cursor"),
	dotDirAdapter("codex", "Codex", "codex", ".codex"),
	dotDirAdapter("hermes", "Hermes", "hermes", ".hermes"),
}

// All returns every registered adapter.
func All() []Adapter { return append([]Adapter(nil), registry...) }

// Get returns the adapter with the given name, or ok=false.
func Get(name string) (Adapter, bool) {
	for _, a := range registry {
		if a.Name == name {
			return a, true
		}
	}
	return Adapter{}, false
}

// Detected returns adapters that appear installed locally.
func Detected() []Adapter {
	var out []Adapter
	for _, a := range registry {
		if a.detect() {
			out = append(out, a)
		}
	}
	return out
}
