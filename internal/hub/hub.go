// Package hub manages the agentsmith hub root (~/.agentsmith): its layout,
// bootstrap, and well-known paths. Phase 1 core.
package hub

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DirPerm is the permission for the hub directory tree. 0700: owner-only,
	// no world/group access (security plan 5.4).
	DirPerm os.FileMode = 0o700
	// FilePerm is the permission for sensitive files like skills.json (0600).
	FilePerm os.FileMode = 0o600
)

// Hub describes a resolved agentsmith hub root and its standard subpaths.
type Hub struct {
	Root string
}

// Default resolves the hub root, honoring AGENTSMITH_HOME, else ~/.agentsmith.
func Default() (*Hub, error) {
	if env := os.Getenv("AGENTSMITH_HOME"); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			return nil, fmt.Errorf("resolve AGENTSMITH_HOME: %w", err)
		}
		return &Hub{Root: abs}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &Hub{Root: filepath.Join(home, ".agentsmith")}, nil
}

// ManifestPath returns the path to skills.json.
func (h *Hub) ManifestPath() string { return filepath.Join(h.Root, "skills.json") }

// ConfigPath returns the path to config.json.
func (h *Hub) ConfigPath() string { return filepath.Join(h.Root, "config.json") }

// LockPath returns the path to the cross-process lock file.
func (h *Hub) LockPath() string { return filepath.Join(h.Root, ".lock") }

// SkillsDir returns the directory holding cloned skill repos.
func (h *Hub) SkillsDir() string { return filepath.Join(h.Root, "skills") }

// CacheDir returns the scratch/clone cache directory.
func (h *Hub) CacheDir() string { return filepath.Join(h.Root, "cache") }

// LogsDir returns the logs directory.
func (h *Hub) LogsDir() string { return filepath.Join(h.Root, "logs") }

// BinDir returns the directory holding the agentsmith binary / PATH shim.
func (h *Hub) BinDir() string { return filepath.Join(h.Root, "bin") }

// SkillPath returns the hub clone path for a named skill.
func (h *Hub) SkillPath(name string) string { return filepath.Join(h.SkillsDir(), name) }

// Bootstrap creates the hub directory tree with restrictive permissions.
// Idempotent: existing dirs are left in place but re-chmod'd to DirPerm.
func (h *Hub) Bootstrap() error {
	dirs := []string{h.Root, h.SkillsDir(), h.CacheDir(), h.LogsDir(), h.BinDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, DirPerm); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
		// MkdirAll respects umask; force the intended perm explicitly.
		if err := os.Chmod(d, DirPerm); err != nil {
			return fmt.Errorf("chmod %s: %w", d, err)
		}
	}
	return nil
}
