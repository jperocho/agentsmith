package cmd

import (
	"flag"
	"fmt"

	"github.com/jperocho/agentsmith/internal/fsx"
	"github.com/jperocho/agentsmith/internal/gitx"
	"github.com/jperocho/agentsmith/internal/manifest"
	"github.com/jperocho/agentsmith/internal/security"
)

// Update fetches the latest commit on a skill's tracked ref, re-resolves the
// version, and re-syncs stale copy installs (plan 7.update). Symlink installs
// need no re-copy — they point at the hub clone. --ref re-points the skill at a
// new tag/branch.
func (a *App) Update(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip the copy-resync confirmation prompt")
	newRef := fs.String("ref", "", "re-point the skill at a new tag/branch")
	allowScripts := fs.Bool("allow-scripts", false, "keep executable bits in the hub clone for this update")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: agentsmith update <skill> [--ref tag|branch] [--allow-scripts] [--yes]")
	}
	name := rest[0]
	if !gitx.Available() {
		return gitx.ErrGitMissing
	}

	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		s, err := m.Get(name)
		if err != nil {
			return false, err
		}
		repoDir := a.Hub.SkillPath(name)
		oldCommit := s.Commit

		fetchRef := s.ResolvedRef
		if *newRef != "" {
			fetchRef = *newRef
		}

		// 1. Fetch the tracked (or newly requested) ref.
		fmt.Printf("Fetching %s%s ...\n", s.Repo, refSuffix(fetchRef))
		if err := gitx.Fetch(repoDir, fetchRef); err != nil {
			return false, err
		}
		newCommit, err := gitx.RemoteCommit(repoDir)
		if err != nil {
			return false, err
		}
		if newCommit == oldCommit && *newRef == "" {
			fmt.Printf("%s already up to date at %s\n", name, shortCommit(oldCommit))
			return false, nil
		}

		// 2. Show changed files. On a shallow clone the old commit may be
		// unreachable (e.g. force-push); flag that rather than failing.
		if files, err := gitx.ChangedFiles(repoDir, oldCommit, newCommit); err != nil {
			fmt.Printf("note: cannot diff %s..%s (shallow clone or rewritten history — possible force-push)\n", shortCommit(oldCommit), shortCommit(newCommit))
		} else if len(files) > 0 {
			fmt.Printf("Changed files (%d):\n", len(files))
			for _, f := range files {
				fmt.Printf("  %s\n", f)
			}
		}

		// 3. Fast-forward (hard reset) the hub clone to the new commit.
		if err := gitx.ResetHard(repoDir, newCommit); err != nil {
			return false, err
		}

		// 4. Re-scan, sanitize, checksum the new tree.
		if err := security.ScanTree(repoDir); err != nil {
			return false, err
		}
		keepScripts := s.AllowScripts || *allowScripts
		if !keepScripts {
			if stripped, err := fsx.StripExecBits(repoDir); err != nil {
				return false, err
			} else if len(stripped) > 0 {
				fmt.Printf("Stripped executable bit from %d file(s)\n", len(stripped))
			}
		}
		checksum, err := security.ChecksumTree(repoDir)
		if err != nil {
			return false, err
		}

		// 5. Update manifest version fields (and tracking if re-pointed).
		s.Commit = newCommit
		s.Checksum = checksum
		s.FetchedAt = manifest.Now()
		if *allowScripts {
			s.AllowScripts = true
		}
		if *newRef != "" {
			s.ResolvedRef = *newRef
			if gitx.IsTag(repoDir, *newRef) {
				s.Tracking = manifest.TrackPinned
			} else {
				s.Tracking = manifest.TrackBranch
			}
			fmt.Printf("Re-pointed %s to %s [%s]\n", name, *newRef, s.Tracking)
		}
		fmt.Printf("Updated %s: %s -> %s\n", name, shortCommit(oldCommit), shortCommit(newCommit))

		// 6. Re-sync stale COPY installs (overwrites agent-side files — gated by
		// a confirmation unless --yes / non-interactive). Symlink installs are
		// already current.
		var stale []*manifest.Install
		for i := range s.Installs {
			if s.Installs[i].Mode == "copy" && s.Installs[i].InstalledCommit != newCommit {
				stale = append(stale, &s.Installs[i])
			}
		}
		if len(stale) > 0 {
			if !confirm(fmt.Sprintf("Re-sync %d copy install(s) with new content?", len(stale)), *yes) {
				fmt.Println("Skipped copy re-sync; those installs are now STALE (run update again to sync).")
				return true, nil
			}
			for _, in := range stale {
				skipped, err := fsx.CopyTreeAtomic(repoDir, in.Target)
				if err != nil {
					return true, fmt.Errorf("re-sync %s install at %s: %w", in.Agent, in.Target, err)
				}
				if len(skipped) > 0 {
					fmt.Printf("note: skipped %d symlink(s) in copy mode for %s install\n", len(skipped), in.Agent)
				}
				in.InstalledCommit = newCommit
				in.InstalledAt = manifest.Now()
				fmt.Printf("Re-synced copy install: %s/%s -> %s\n", in.Agent, in.Scope, in.Target)
			}
		}
		return true, nil
	})
}
