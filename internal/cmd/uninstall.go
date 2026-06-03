package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/jperocho/agentsmith/internal/agent"
	"github.com/jperocho/agentsmith/internal/manifest"
)

// Uninstall removes a skill's install from an agent (plan: uninstall). Without
// --agent it removes every install of the skill.
func (a *App) Uninstall(args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	global := fs.Bool("global", false, "target the global-scope install")
	project := fs.Bool("project", false, "target the project-scope install")
	agentName := fs.String("agent", "", "agent name to uninstall from (default: all)")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: agentsmith uninstall <skill> [--agent name] [--global|--project]")
	}
	name := rest[0]

	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		s, err := m.Get(name)
		if err != nil {
			return false, err
		}
		var kept []manifest.Install
		removed := 0
		for _, in := range s.Installs {
			if !matchInstall(in, *agentName, *global, *project) {
				kept = append(kept, in)
				continue
			}
			if err := os.RemoveAll(in.Target); err != nil {
				fmt.Printf("warning: could not remove %s: %v\n", in.Target, err)
				kept = append(kept, in)
				continue
			}
			fmt.Printf("Uninstalled %s from %s/%s (%s)\n", name, in.Agent, in.Scope, in.Target)
			removed++
		}
		if removed == 0 {
			return false, fmt.Errorf("no matching install found for %q", name)
		}
		s.Installs = kept
		return true, nil
	})
}

func matchInstall(in manifest.Install, agentName string, global, project bool) bool {
	if agentName != "" {
		if _, ok := agent.Get(agentName); !ok {
			// Unknown name simply won't match anything.
		}
		if in.Agent != agentName {
			return false
		}
	}
	if global && in.Scope != string(agent.Global) {
		return false
	}
	if project && in.Scope != string(agent.Project) {
		return false
	}
	return true
}
