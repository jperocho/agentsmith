package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jperocho/agentsmith/internal/fsx"
	"github.com/jperocho/agentsmith/internal/gitx"
	"github.com/jperocho/agentsmith/internal/manifest"
	"github.com/jperocho/agentsmith/internal/security"
)

// Get clones a skill repo into the hub and registers it (plan 7.get). It does
// NOT install into any agent.
func (a *App) Get(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	allowScripts := fs.Bool("allow-scripts", false, "keep executable bits in the hub clone (default strips them)")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: agentsmith get <repo>[@ref] [--allow-scripts]")
	}
	if !gitx.Available() {
		return gitx.ErrGitMissing
	}
	spec, err := parseRepo(rest[0])
	if err != nil {
		return err
	}

	return a.withLockedManifest(func(m *manifest.Manifest) (bool, error) {
		if _, err := m.Get(spec.Name); err == nil {
			return false, fmt.Errorf("skill %q already in hub; run: agentsmith update %s", spec.Name, spec.Name)
		}

		// 1. Clone into a cache scratch dir (security: stage before registering).
		scratch, err := os.MkdirTemp(a.Hub.CacheDir(), "get-*")
		if err != nil {
			return false, fmt.Errorf("create scratch dir: %w", err)
		}
		defer os.RemoveAll(scratch)
		clonePath := filepath.Join(scratch, spec.Name)

		if strings.HasPrefix(spec.URL, "http://") {
			fmt.Printf("warning: %s uses plaintext http:// — content is unauthenticated and MITM-able; prefer https://\n", spec.URL)
		}
		fmt.Printf("Cloning %s%s ...\n", spec.URL, refSuffix(spec.Ref))
		if err := gitx.Clone(spec.URL, spec.Ref, clonePath); err != nil {
			return false, err
		}

		// 2. Resolve exact commit + ref.
		commit, err := gitx.HeadCommit(clonePath)
		if err != nil {
			return false, err
		}
		resolvedRef := spec.Ref
		tracking := manifest.TrackBranch
		if spec.Ref == "" {
			if r, err := gitx.CurrentRef(clonePath); err == nil {
				resolvedRef = r
			}
		} else if gitx.IsTag(clonePath, spec.Ref) {
			tracking = manifest.TrackPinned
		}

		// 3. Security scan first, then sanitize and checksum.
		if err := security.ScanTree(clonePath); err != nil {
			return false, err
		}
		// Warn (don't fail) if the repo has no root SKILL.md — usually wrong repo.
		if _, err := os.Stat(filepath.Join(clonePath, "SKILL.md")); err != nil {
			fmt.Printf("warning: no SKILL.md at repo root — is %q a skill repo?\n", spec.Name)
		}
		// Strip executable bits unless the user opted in (security 5.3). Done
		// before checksum so the recorded checksum reflects the sanitized tree.
		if !*allowScripts {
			if stripped, err := fsx.StripExecBits(clonePath); err != nil {
				return false, err
			} else if len(stripped) > 0 {
				fmt.Printf("Stripped executable bit from %d file(s) (use --allow-scripts to keep)\n", len(stripped))
			}
		}
		checksum, err := security.ChecksumTree(clonePath)
		if err != nil {
			return false, err
		}

		// 4. Move into skills/<name>/ atomically.
		dest := a.Hub.SkillPath(spec.Name)
		if err := os.RemoveAll(dest); err != nil {
			return false, fmt.Errorf("clear destination: %w", err)
		}
		if err := os.Rename(clonePath, dest); err != nil {
			return false, fmt.Errorf("move into hub: %w", err)
		}

		// 5. Register in manifest.
		m.Skills[spec.Name] = &manifest.Skill{
			Repo:         spec.URL,
			ResolvedRef:  resolvedRef,
			Tracking:     tracking,
			Commit:       commit,
			FetchedAt:    manifest.Now(),
			Path:         filepath.Join("skills", spec.Name),
			Checksum:     checksum,
			AllowScripts: *allowScripts,
			Installs:     nil,
		}

		fmt.Printf("Staged %s @ %s [%s] (%s)\n", spec.Name, displayRef(resolvedRef), tracking, shortCommit(commit))
		fmt.Printf("Next: agentsmith install %s\n", spec.Name)
		return true, nil
	})
}

func refSuffix(ref string) string {
	if ref == "" {
		return ""
	}
	return "@" + ref
}

func displayRef(ref string) string {
	if ref == "" {
		return "(default branch)"
	}
	return ref
}

func shortCommit(c string) string {
	if len(c) > 10 {
		return c[:10]
	}
	return c
}
