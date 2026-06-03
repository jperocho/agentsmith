// Package gitx wraps the system `git` binary. The plan (section 9) chooses
// shelling out to git over a library: simplest, and it reuses the user's ssh
// agent and credential helpers. TLS verification is left at git defaults and
// insecure flags are never passed (security 5.1).
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGitMissing indicates the git binary was not found on PATH.
var ErrGitMissing = errors.New("git not found on PATH")

// Available reports whether the git binary is usable.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// run executes git with args in dir (empty dir = inherit cwd) and returns
// trimmed stdout. Stderr is folded into the error for diagnostics.
func run(dir string, args ...string) (string, error) {
	if !Available() {
		return "", ErrGitMissing
	}
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// Clone clones url into dest. If ref is non-empty it is checked out (branch or
// tag). A shallow clone (depth 1) is used to limit download size; the exact
// commit is resolved afterward via HeadCommit.
func Clone(url, ref, dest string) error {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "--", url, dest)
	if _, err := run("", args...); err != nil {
		return err
	}
	return nil
}

// HeadCommit returns the full commit SHA of HEAD in repoDir.
func HeadCommit(repoDir string) (string, error) {
	return run(repoDir, "rev-parse", "HEAD")
}

// CurrentRef returns the symbolic ref (branch/tag) HEAD points at, or the short
// commit if detached. Best-effort; used to populate resolvedRef.
func CurrentRef(repoDir string) (string, error) {
	if ref, err := run(repoDir, "symbolic-ref", "--short", "-q", "HEAD"); err == nil && ref != "" {
		return ref, nil
	}
	// Detached HEAD (e.g. a checked-out tag): try describing exact tags.
	if tag, err := run(repoDir, "describe", "--tags", "--exact-match"); err == nil && tag != "" {
		return tag, nil
	}
	return run(repoDir, "rev-parse", "--short", "HEAD")
}

// IsTag reports whether ref names an existing tag in repoDir.
func IsTag(repoDir, ref string) bool {
	if ref == "" {
		return false
	}
	out, err := run(repoDir, "tag", "-l", ref)
	return err == nil && out == ref
}

// Fetch fetches updates for ref (or all if empty) without merging.
func Fetch(repoDir, ref string) error {
	args := []string{"fetch", "--depth", "1", "origin"}
	if ref != "" {
		args = append(args, ref)
	}
	_, err := run(repoDir, args...)
	return err
}

// RemoteCommit returns the commit SHA fetched by the preceding Fetch call, i.e.
// the tip of the ref that Fetch resolved into FETCH_HEAD. Call only after Fetch.
func RemoteCommit(repoDir string) (string, error) {
	return run(repoDir, "rev-parse", "FETCH_HEAD")
}

// ResetHard hard-resets repoDir's working tree to commit (used by update to
// fast-forward the hub clone to the fetched commit).
func ResetHard(repoDir, commit string) error {
	_, err := run(repoDir, "reset", "--hard", commit)
	return err
}

// ChangedFiles returns files differing between two commits in repoDir.
func ChangedFiles(repoDir, from, to string) ([]string, error) {
	out, err := run(repoDir, "diff", "--name-only", from, to)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
