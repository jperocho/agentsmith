package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/jperocho/agentsmith/internal/manifest"
)

// Remove deletes a skill from the hub entirely. By default it also removes any
// installs it placed into agents; --keep-installs leaves those files in place
// (untracked afterward).
func (a *App) Remove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	keepInstalls := fs.Bool("keep-installs", false, "leave installed agent files in place (just drop tracking)")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: agentsmith remove <skill> [--keep-installs]")
	}
	name := rest[0]
	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		s, err := m.Get(name)
		if err != nil {
			return false, err
		}
		// Remove install artifacts first (best-effort, reported) unless kept.
		if *keepInstalls {
			if len(s.Installs) > 0 {
				fmt.Printf("Keeping %d install(s) in place (now untracked).\n", len(s.Installs))
			}
		} else {
			for _, in := range s.Installs {
				if err := os.RemoveAll(in.Target); err != nil {
					fmt.Printf("warning: could not remove install %s: %v\n", in.Target, err)
				} else {
					fmt.Printf("Removed install: %s (%s/%s)\n", in.Target, in.Agent, in.Scope)
				}
			}
		}
		// Remove hub clone.
		if err := os.RemoveAll(a.Hub.SkillPath(name)); err != nil {
			return false, fmt.Errorf("remove hub clone: %w", err)
		}
		delete(m.Skills, name)
		fmt.Printf("Removed skill %q from hub.\n", name)
		return true, nil
	})
}
