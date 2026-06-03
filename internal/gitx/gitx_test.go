package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs a git command in dir with a deterministic identity, failing the test
// on error. Used only to build fixture repos — the code under test is gitx.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func requireGit(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skip("git not on PATH")
	}
}

// initOrigin builds a one-commit origin repo on branch main and returns its path.
func initOrigin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "first")
	return dir
}

func TestAvailable(t *testing.T) {
	if Available() != (func() bool { _, err := exec.LookPath("git"); return err == nil }()) {
		t.Error("Available disagrees with LookPath")
	}
}

func TestCloneHeadCommitCurrentRef(t *testing.T) {
	requireGit(t)
	origin := initOrigin(t)
	want := git(t, origin, "rev-parse", "HEAD")

	dest := filepath.Join(t.TempDir(), "clone")
	if err := Clone(origin, "", dest); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	got, err := HeadCommit(dest)
	if err != nil {
		t.Fatalf("HeadCommit: %v", err)
	}
	if got != want {
		t.Errorf("HeadCommit = %q, want %q", got, want)
	}
	if ref, err := CurrentRef(dest); err != nil || ref != "main" {
		t.Errorf("CurrentRef = %q,%v want \"main\"", ref, err)
	}
}

func TestIsTag(t *testing.T) {
	requireGit(t)
	origin := initOrigin(t)
	git(t, origin, "tag", "v1.0.0")

	dest := filepath.Join(t.TempDir(), "clone")
	if err := Clone(origin, "v1.0.0", dest); err != nil {
		t.Fatalf("Clone tag: %v", err)
	}
	if !IsTag(dest, "v1.0.0") {
		t.Error("IsTag(v1.0.0) = false, want true")
	}
	if IsTag(dest, "nope") {
		t.Error("IsTag(nope) = true, want false")
	}
	if IsTag(dest, "") {
		t.Error("IsTag(\"\") = true, want false")
	}
}

func TestFetchRemoteCommitResetHard(t *testing.T) {
	requireGit(t)
	origin := initOrigin(t)

	dest := filepath.Join(t.TempDir(), "clone")
	if err := Clone(origin, "", dest); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Advance origin/main by one commit.
	if err := os.WriteFile(filepath.Join(origin, "SKILL.md"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, origin, "commit", "-q", "-am", "second")
	want := git(t, origin, "rev-parse", "HEAD")

	if err := Fetch(dest, "main"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	rc, err := RemoteCommit(dest)
	if err != nil {
		t.Fatalf("RemoteCommit: %v", err)
	}
	if rc != want {
		t.Errorf("RemoteCommit = %q, want origin HEAD %q", rc, want)
	}
	if err := ResetHard(dest, rc); err != nil {
		t.Fatalf("ResetHard: %v", err)
	}
	if head, _ := HeadCommit(dest); head != want {
		t.Errorf("after ResetHard HeadCommit = %q, want %q", head, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "SKILL.md")); string(b) != "v2" {
		t.Errorf("working tree not reset to v2, got %q", b)
	}
}

func TestChangedFiles(t *testing.T) {
	requireGit(t)
	origin := initOrigin(t)
	dest := filepath.Join(t.TempDir(), "clone")
	if err := Clone(origin, "", dest); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	c1, _ := HeadCommit(dest)
	// Commit a new file inside the clone so both commits are locally reachable
	// (the shallow clone has no older history to diff against otherwise).
	if err := os.WriteFile(filepath.Join(dest, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dest, "add", ".")
	git(t, dest, "commit", "-q", "-m", "add new")
	c2, _ := HeadCommit(dest)

	files, err := ChangedFiles(dest, c1, c2)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(files) != 1 || files[0] != "new.txt" {
		t.Errorf("ChangedFiles = %v, want [new.txt]", files)
	}

	// No diff between a commit and itself → empty, no error.
	none, err := ChangedFiles(dest, c2, c2)
	if err != nil {
		t.Fatalf("ChangedFiles(same): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("ChangedFiles(same) = %v, want empty", none)
	}
}
