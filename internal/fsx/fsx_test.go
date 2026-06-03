package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripExecBits(t *testing.T) {
	d := t.TempDir()
	exe := filepath.Join(d, "run.sh")
	plain := filepath.Join(d, "a.txt")
	os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(plain, []byte("x"), 0o644)

	stripped, err := StripExecBits(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(stripped) != 1 {
		t.Errorf("stripped %d files, want 1", len(stripped))
	}
	if fi, _ := os.Stat(exe); fi.Mode().Perm()&0o111 != 0 {
		t.Errorf("exec bit not cleared: %o", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(plain); fi.Mode().Perm() != 0o644 {
		t.Errorf("plain file perms altered: %o", fi.Mode().Perm())
	}
}

func TestCopyTreeAtomic(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "x.txt"), []byte("deep"), 0o644)
	os.MkdirAll(filepath.Join(src, ".git"), 0o755)
	os.WriteFile(filepath.Join(src, ".git", "HEAD"), []byte("ref"), 0o644)

	dst := filepath.Join(t.TempDir(), "out")
	if _, err := CopyTreeAtomic(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "x.txt")); string(b) != "deep" {
		t.Errorf("nested file not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git should not be copied")
	}
}

func TestCopyTreeAtomicReplaces(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "v.txt"), []byte("new"), 0o644)
	dst := filepath.Join(t.TempDir(), "out")
	os.MkdirAll(dst, 0o755)
	os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("old"), 0o644)

	if _, err := CopyTreeAtomic(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("stale file survived replace")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "v.txt")); string(b) != "new" {
		t.Errorf("new content missing")
	}
}

func TestCopyTreeAtomicSkipsSymlinks(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("hi"), 0o644)
	if err := os.Symlink("real.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	skipped, err := CopyTreeAtomic(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0] != "link.txt" {
		t.Errorf("skipped = %v, want [link.txt]", skipped)
	}
	if _, err := os.Lstat(filepath.Join(dst, "link.txt")); !os.IsNotExist(err) {
		t.Errorf("symlink should not be copied into install")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "real.txt")); string(b) != "hi" {
		t.Errorf("regular file not copied")
	}
}

func TestSymlinkAtomic(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "f"), []byte("x"), 0o644)
	dst := filepath.Join(t.TempDir(), "link")
	if err := SymlinkAtomic(src, dst); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	target, err := os.Readlink(dst)
	if err != nil || target != src {
		t.Errorf("Readlink = %q,%v want %q", target, err, src)
	}
	// Replacing an existing link should succeed.
	if err := SymlinkAtomic(src, dst); err != nil {
		t.Errorf("re-symlink failed: %v", err)
	}
}
