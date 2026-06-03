// Package security implements the cheap-but-critical guards from the plan's
// security model (section 5): content tree checksums, a pre-register scan that
// rejects path traversal and symlink escapes, install-target allowlisting, and
// atomic copy/symlink installs that never write outside known roots.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MaxFileSize caps a single skill file (security 5.2: zip-bomb / disk fill).
const MaxFileSize int64 = 50 << 20 // 50 MiB

// MaxTreeSize caps the total skill tree size.
const MaxTreeSize int64 = 250 << 20 // 250 MiB

// ChecksumTree computes a sha256 over the sorted file tree under root,
// returning "sha256:<hex>". The hash covers each file's relative path and
// content, so renames and edits both change it (plan 5.2). The .git directory
// is excluded — it is VCS metadata, not skill content.
func ChecksumTree(root string) (string, error) {
	var rels []string
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
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Strings(rels)

	h := sha256.New()
	for _, rel := range rels {
		// Path component, normalized to forward slashes for cross-platform stability.
		io.WriteString(h, filepath.ToSlash(rel))
		h.Write([]byte{0})
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			return "", fmt.Errorf("open %s: %w", rel, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", fmt.Errorf("hash %s: %w", rel, err)
		}
		f.Close()
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ScanTree validates an untrusted skill tree before it is registered or copied
// (security 5.2). It rejects:
//   - symlinks whose target escapes the tree root,
//   - files exceeding MaxFileSize or a cumulative MaxTreeSize.
//
// filepath.Walk does not follow symlinks, so a malicious symlink is inspected,
// not traversed. The .git directory is skipped.
func ScanTree(root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	var total int64
	return filepath.Walk(absRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		// Symlink: ensure its (possibly relative) target stays inside the tree.
		if info.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", p, err)
			}
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(filepath.Dir(p), dest)
			}
			dest = filepath.Clean(dest)
			if !within(absRoot, dest) {
				return fmt.Errorf("rejected: symlink %q escapes skill dir (-> %q)", p, dest)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("rejected: %q is not a regular file (mode %s)", p, info.Mode())
		}
		if info.Size() > MaxFileSize {
			return fmt.Errorf("rejected: %q exceeds per-file cap (%d > %d bytes)", p, info.Size(), MaxFileSize)
		}
		total += info.Size()
		if total > MaxTreeSize {
			return fmt.Errorf("rejected: skill tree exceeds total cap (%d bytes)", MaxTreeSize)
		}
		return nil
	})
}

// ResolveTarget canonicalizes a desired install path and enforces that it sits
// under one of allowedRoots (security 5.4: refuse to write outside known agent
// config dirs). It resolves symlinks on the existing parent chain to defeat a
// symlinked target pointing elsewhere. Returns the cleaned absolute path.
func ResolveTarget(target string, allowedRoots []string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)

	// Resolve the deepest existing ancestor's real path, then re-append the
	// not-yet-existing tail. This catches a parent that is itself a symlink
	// escaping the allowlist.
	real := resolveExistingAncestor(abs)

	for _, root := range allowedRoots {
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rootReal := resolveExistingAncestor(filepath.Clean(rootAbs))
		if within(rootReal, real) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("refused: target %q is outside allowed agent roots", abs)
}

// resolveExistingAncestor walks up p until it finds an existing path, resolves
// its symlinks, and rejoins the non-existent tail.
func resolveExistingAncestor(p string) string {
	tail := ""
	cur := p
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if tail == "" {
				return resolved
			}
			return filepath.Join(resolved, tail)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached root with nothing existing; return original cleaned path.
			return p
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// within reports whether path is root itself or nested under root.
func within(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
