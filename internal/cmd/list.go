package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jperocho/agentsmith/internal/manifest"
)

// List prints hub skills with versions and install targets (plan: list).
func (a *App) List(args []string) error {
	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		names := m.Names()
		if len(names) == 0 {
			fmt.Println("No skills in hub. Add one: agentsmith get <repo>")
			return false, nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "SKILL\tREF\tCOMMIT\tINSTALLS")
		for _, name := range names {
			s := m.Skills[name]
			installs := fmt.Sprintf("%d", len(s.Installs))
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, displayRef(s.ResolvedRef), shortCommit(s.Commit), installs)
			for _, in := range s.Installs {
				fmt.Fprintf(w, "  └─ %s\t%s\t%s\t%s\n", in.Agent, in.Scope, in.Mode, in.Target)
			}
		}
		return false, w.Flush()
	})
}
