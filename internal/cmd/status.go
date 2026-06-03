package cmd

import (
	"fmt"
	"os"

	"github.com/jperocho/agentsmith/internal/manifest"
)

// Status shows install state and drift for a skill (plan: status). A copy
// install is STALE when its installedCommit differs from the hub commit, or
// when its target is missing. Symlink installs always reflect the hub.
func (a *App) Status(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agentsmith status <skill>")
	}
	name := args[0]
	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		s, err := m.Get(name)
		if err != nil {
			return false, err
		}
		fmt.Printf("Skill:    %s\n", name)
		fmt.Printf("Repo:     %s\n", s.Repo)
		fmt.Printf("Ref:      %s (%s)\n", displayRef(s.ResolvedRef), s.Tracking)
		fmt.Printf("Commit:   %s\n", s.Commit)
		fmt.Printf("Checksum: %s\n", s.Checksum)
		fmt.Printf("Fetched:  %s\n", s.FetchedAt)
		if len(s.Installs) == 0 {
			fmt.Println("Installs: none")
			return false, nil
		}
		fmt.Println("Installs:")
		for _, in := range s.Installs {
			fmt.Printf("  - %s/%s [%s] %s\n      %s\n", in.Agent, in.Scope, in.Mode, driftState(name, s, in), in.Target)
		}
		return false, nil
	})
}

// driftState classifies an install relative to the hub.
func driftState(name string, s *manifest.Skill, in manifest.Install) string {
	if _, err := os.Lstat(in.Target); err != nil {
		return "MISSING (target not present)"
	}
	if in.Mode == "symlink" {
		return "UP-TO-DATE (symlink)"
	}
	if in.InstalledCommit != s.Commit {
		return fmt.Sprintf("STALE (installed %s, hub %s) — run: agentsmith update %s", shortCommit(in.InstalledCommit), shortCommit(s.Commit), name)
	}
	return "UP-TO-DATE"
}
