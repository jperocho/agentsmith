package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jperocho/agentsmith/internal/agent"
	"github.com/jperocho/agentsmith/internal/fsx"
	"github.com/jperocho/agentsmith/internal/manifest"
	"github.com/jperocho/agentsmith/internal/security"
)

// stringList is a repeatable / comma-separated string flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			*s = append(*s, p)
		}
	}
	return nil
}

// Install installs a hub skill into one or more agents (plan 7.install).
func (a *App) Install(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	global := fs.Bool("global", false, "install to the user-wide agent config")
	mode := fs.String("mode", "", "install mode: copy | symlink (default: symlink, falls back to copy)")
	var agents stringList
	fs.Var(&agents, "agent", "agent name(s) to install into (repeatable or comma-separated)")
	yes := fs.Bool("yes", false, "non-interactive: assume defaults, no prompts")
	dryRun := fs.Bool("dry-run", false, "print resolved target(s) and mode; write nothing")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: agentsmith install <skill> [--global] [--mode copy|symlink] [--agent name]...")
	}
	name := rest[0]

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	scope := agent.Project
	if *global {
		scope = agent.Global
	}

	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		s, err := m.Get(name)
		if err != nil {
			return false, err
		}
		srcDir := a.Hub.SkillPath(name)
		if _, err := os.Stat(srcDir); err != nil {
			return false, fmt.Errorf("hub clone missing for %q at %s: %w", name, srcDir, err)
		}

		// Select target agents.
		targets, err := selectAgents(agents, *yes)
		if err != nil {
			return false, err
		}

		changed := false
		for _, ad := range targets {
			target, err := ad.Dir(scope, cwd, name)
			if err != nil {
				return changed, err
			}
			// Allowlist enforcement (security 5.4).
			resolved, err := security.ResolveTarget(target, ad.AllowedRoots(cwd))
			if err != nil {
				return changed, err
			}

			useMode, err := resolveMode(*mode, a.Cfg.DefaultMode, target)
			if err != nil {
				return changed, err
			}

			if *dryRun {
				fmt.Printf("[dry-run] would install %s -> %s [%s/%s/%s]\n", name, resolved, ad.Name, scope, useMode)
				continue
			}

			switch useMode {
			case "symlink":
				if err := fsx.SymlinkAtomic(srcDir, resolved); err != nil {
					return changed, err
				}
			case "copy":
				if err := security.ScanTree(srcDir); err != nil {
					return changed, err
				}
				if err := fsx.CopyTreeAtomic(srcDir, resolved); err != nil {
					return changed, err
				}
			}

			// Record / update install entry in manifest.
			entry := manifest.Install{
				Agent:           ad.Name,
				Scope:           string(scope),
				Target:          resolved,
				Mode:            useMode,
				InstalledCommit: s.Commit,
				InstalledAt:     manifest.Now(),
			}
			if idx := s.FindInstall(ad.Name, string(scope)); idx >= 0 {
				s.Installs[idx] = entry
			} else {
				s.Installs = append(s.Installs, entry)
			}
			changed = true
			fmt.Printf("Installed %s -> %s [%s/%s/%s]\n", name, resolved, ad.Name, scope, useMode)
		}
		if !changed && !*dryRun {
			fmt.Println("No agents selected; nothing installed.")
		}
		return changed, nil
	})
}

// selectAgents resolves the target adapters: from --agent flags if given, else
// an interactive multi-select over detected agents (or all detected if --yes).
func selectAgents(requested stringList, yes bool) ([]agent.Adapter, error) {
	if len(requested) > 0 {
		var out []agent.Adapter
		for _, name := range requested {
			ad, ok := agent.Get(name)
			if !ok {
				return nil, fmt.Errorf("unknown agent %q", name)
			}
			out = append(out, ad)
		}
		return out, nil
	}

	detected := agent.Detected()
	if len(detected) == 0 {
		return nil, fmt.Errorf("no supported agents detected; specify one with --agent")
	}
	if yes || len(detected) == 1 {
		return detected, nil
	}

	fmt.Println("Detected agents:")
	for i, ad := range detected {
		fmt.Printf("  [%d] %s (%s)\n", i+1, ad.Title, ad.Name)
	}
	fmt.Print("Select agent(s) by number (comma-separated, 'a' for all): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "a" || line == "all" || line == "" {
		return detected, nil
	}
	var out []agent.Adapter
	for _, tok := range strings.Split(line, ",") {
		tok = strings.TrimSpace(tok)
		var idx int
		if _, err := fmt.Sscanf(tok, "%d", &idx); err != nil || idx < 1 || idx > len(detected) {
			return nil, fmt.Errorf("invalid selection %q", tok)
		}
		out = append(out, detected[idx-1])
	}
	return out, nil
}

// resolveMode picks copy vs symlink. Precedence: explicit --mode, then
// config.json defaultMode, then auto (symlink where supported, else copy —
// plan section 6).
func resolveMode(requested, configDefault, target string) (string, error) {
	switch requested {
	case "copy", "symlink":
		return requested, nil
	case "":
		// fall through to config / auto
	default:
		return "", fmt.Errorf("invalid --mode %q (use copy or symlink)", requested)
	}
	switch configDefault {
	case "copy", "symlink":
		return configDefault, nil
	}
	parent := parentDir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	if fsx.SymlinkSupported(parent) {
		return "symlink", nil
	}
	return "copy", nil
}

func parentDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
