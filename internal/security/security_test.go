package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithin(t *testing.T) {
	cases := []struct {
		root, path string
		want       bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/bc", false},
		{"/a/b", "/a", false},
		{"/a/b", "/a/b/../c", false},
	}
	for _, c := range cases {
		if got := within(c.root, c.path); got != c.want {
			t.Errorf("within(%q,%q)=%v want %v", c.root, c.path, got, c.want)
		}
	}
}

func TestResolveTargetAllowlist(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "ok")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatal(err)
	}

	good := filepath.Join(allowed, "skill")
	if _, err := ResolveTarget(good, []string{allowed}); err != nil {
		t.Errorf("ResolveTarget(in-allowlist) errored: %v", err)
	}

	bad := filepath.Join(root, "elsewhere", "skill")
	if _, err := ResolveTarget(bad, []string{allowed}); err == nil {
		t.Errorf("ResolveTarget(outside) = nil error, want refusal")
	}
}

func TestResolveTargetSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "agentdir")
	outside := filepath.Join(root, "outside")
	os.MkdirAll(allowed, 0o755)
	os.MkdirAll(outside, 0o755)

	// A symlink inside the allowed dir pointing outside it.
	link := filepath.Join(allowed, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	// Target under the symlink resolves outside the allowlist -> must be refused.
	if _, err := ResolveTarget(filepath.Join(link, "skill"), []string{allowed}); err == nil {
		t.Errorf("ResolveTarget via escaping symlink = nil error, want refusal")
	}
}

func TestScanTreeRejectsEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "ok.txt"), []byte("hi"), 0o644)
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "steal")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := ScanTree(root); err == nil {
		t.Errorf("ScanTree = nil, want rejection of escaping symlink")
	}
}

func TestScanTreeAcceptsCleanTree(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# s"), 0o644)
	os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("a"), 0o644)
	if err := ScanTree(root); err != nil {
		t.Errorf("ScanTree(clean) errored: %v", err)
	}
}

func TestChecksumTreeStableAndSensitive(t *testing.T) {
	mk := func() string {
		d := t.TempDir()
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("alpha"), 0o644)
		os.WriteFile(filepath.Join(d, "b.txt"), []byte("beta"), 0o644)
		return d
	}
	d1 := mk()
	d2 := mk()
	s1, err := ChecksumTree(d1)
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := ChecksumTree(d2)
	if s1 != s2 {
		t.Errorf("identical trees got different checksums: %s vs %s", s1, s2)
	}
	// Mutate content -> checksum changes.
	os.WriteFile(filepath.Join(d2, "a.txt"), []byte("ALPHA"), 0o644)
	s3, _ := ChecksumTree(d2)
	if s3 == s1 {
		t.Errorf("checksum unchanged after content edit")
	}
}

func TestChecksumIgnoresGitDir(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "a.txt"), []byte("x"), 0o644)
	base, _ := ChecksumTree(d)
	os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	os.WriteFile(filepath.Join(d, ".git", "HEAD"), []byte("ref"), 0o644)
	after, _ := ChecksumTree(d)
	if base != after {
		t.Errorf("checksum changed when .git added; should be ignored")
	}
}
