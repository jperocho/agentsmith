package cmd

import (
	"fmt"
	"os"

	"github.com/jperocho/agentsmith/internal/agent"
	"github.com/jperocho/agentsmith/internal/gitx"
	"github.com/jperocho/agentsmith/internal/manifest"
	"github.com/jperocho/agentsmith/internal/security"
)

// Doctor detects agents, checks prerequisites, and validates the manifest +
// hub integrity (plan: doctor; security 5.6 reports checksum drift / tampering).
func (a *App) Doctor(args []string) error {
	problems := 0

	fmt.Println("== Environment ==")
	if gitx.Available() {
		fmt.Println("  ok    git present on PATH")
	} else {
		fmt.Println("  FAIL  git not found on PATH (required for get/update)")
		problems++
	}

	fmt.Println("== Hub ==")
	if info, err := os.Stat(a.Hub.Root); err == nil {
		mode := info.Mode().Perm()
		if mode&0o077 != 0 {
			fmt.Printf("  warn  hub %s perms %o are not 0700\n", a.Hub.Root, mode)
		} else {
			fmt.Printf("  ok    hub %s (0700)\n", a.Hub.Root)
		}
	} else {
		fmt.Printf("  FAIL  hub root missing: %v\n", err)
		problems++
	}

	fmt.Println("== Agents ==")
	for _, ad := range agent.All() {
		if ad.Detect() {
			fmt.Printf("  ok    %s detected\n", ad.Title)
		} else {
			fmt.Printf("  --    %s not detected\n", ad.Title)
		}
	}

	fmt.Println("== Skills ==")
	err := a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		if len(m.Skills) == 0 {
			fmt.Println("  (no skills in hub)")
			return false, nil
		}
		for _, name := range m.Names() {
			s := m.Skills[name]
			repoDir := a.Hub.SkillPath(name)
			if _, err := os.Stat(repoDir); err != nil {
				fmt.Printf("  FAIL  %s: hub clone missing (%s)\n", name, repoDir)
				problems++
				continue
			}
			// Integrity: recompute checksum, compare to manifest (5.6 tamper check).
			sum, err := security.ChecksumTree(repoDir)
			if err != nil {
				fmt.Printf("  FAIL  %s: checksum error: %v\n", name, err)
				problems++
			} else if sum != s.Checksum {
				fmt.Printf("  FAIL  %s: checksum mismatch (hub tree changed locally)\n        manifest %s\n        actual   %s\n", name, s.Checksum, sum)
				problems++
			} else {
				fmt.Printf("  ok    %s @ %s (checksum verified)\n", name, shortCommit(s.Commit))
			}
			// Install targets present?
			for _, in := range s.Installs {
				if _, err := os.Lstat(in.Target); err != nil {
					fmt.Printf("  warn  %s: install target missing %s (%s/%s)\n", name, in.Target, in.Agent, in.Scope)
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	fmt.Println()
	if problems == 0 {
		fmt.Println("doctor: no problems found.")
		return nil
	}
	return fmt.Errorf("doctor: %d problem(s) found", problems)
}
