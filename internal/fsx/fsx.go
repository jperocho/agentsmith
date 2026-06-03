// Package fsx provides the filesystem install primitives: atomic copy-tree and
// symlink creation into agent target dirs (security 5.4: copy installs written
// to a temp dir then renamed; never follow a target symlink out of the dir).
package fsx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyTreeAtomic copies the skill tree at src into dst. It first materializes a
// sibling temp dir, then swaps it into place: any existing dst is removed and
// the temp dir renamed over it, so a crash never leaves a half-written install.
// The .git directory is excluded — installs ship skill content, not VCS data.
func CopyTreeAtomic(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, ".agentsmith-install-*")
	if err != nil {
		return fmt.Errorf("create temp install dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(tmp)
		}
	}()

	if err := copyTree(src, tmp); err != nil {
		return err
	}
	// Remove any prior install, then atomically rename the staged tree in.
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove existing target %s: %w", dst, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("swap install into %s: %w", dst, err)
	}
	cleanup = false
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !info.Mode().IsRegular() {
			// Skip symlinks/specials; ScanTree already vetted them as in-tree.
			return nil
		}
		return copyFile(p, filepath.Join(dst, rel), info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// SymlinkAtomic creates a symlink at dst pointing to src, replacing any prior
// install. It writes the link under a temp name then renames it into place so
// the swap is atomic.
func SymlinkAtomic(src, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent %s: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, ".agentsmith-link-*")
	if err != nil {
		return fmt.Errorf("create temp link dir: %w", err)
	}
	linkTmp := filepath.Join(tmp, "link")
	if err := os.Symlink(src, linkTmp); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("create symlink: %w", err)
	}
	if err := os.RemoveAll(dst); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("remove existing target %s: %w", dst, err)
	}
	if err := os.Rename(linkTmp, dst); err != nil {
		os.RemoveAll(tmp)
		return fmt.Errorf("swap symlink into %s: %w", dst, err)
	}
	os.RemoveAll(tmp)
	return nil
}

// StripExecBits clears the executable bits (0111) from every regular file in
// the tree at root. Skill content is untrusted; agentsmith neither runs nor
// ships executables unless the user opts in with --allow-scripts (plan 5.3).
// The .git directory is left untouched. Returns the paths it sanitized.
func StripExecBits(root string) ([]string, error) {
	var stripped []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Mode().Perm()&0o111 != 0 {
			if err := os.Chmod(p, info.Mode().Perm()&^0o111); err != nil {
				return fmt.Errorf("strip exec bit %s: %w", p, err)
			}
			stripped = append(stripped, p)
		}
		return nil
	})
	return stripped, err
}

// SymlinkSupported reports whether symlinks can be created under dir. On
// platforms/filesystems without symlink permission, install falls back to copy.
func SymlinkSupported(dir string) bool {
	tmp, err := os.MkdirTemp(dir, ".agentsmith-symtest-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tmp)
	link := filepath.Join(tmp, "l")
	if err := os.Symlink(tmp, link); err != nil {
		return false
	}
	return true
}
