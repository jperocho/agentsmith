package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "skills.json"))
	if err != nil {
		t.Fatalf("Load missing errored: %v", err)
	}
	if m.SchemaVersion != SchemaVersion || len(m.Skills) != 0 {
		t.Errorf("expected empty manifest, got %+v", m)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills.json")
	m, _ := Load(path)
	m.Skills["demo"] = &Skill{
		Repo:        "https://github.com/o/demo",
		ResolvedRef: "v1",
		Tracking:    TrackPinned,
		Commit:      "abc123",
		Checksum:    "sha256:deadbeef",
		Installs: []Install{
			{Agent: "claude-code", Scope: "project", Mode: "copy", InstalledCommit: "abc123", Target: "/x"},
		},
	}
	if err := m.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Perms must be 0600.
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("manifest perms = %o, want 0600", fi.Mode().Perm())
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := got.Get("demo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Tracking != TrackPinned || s.Commit != "abc123" || len(s.Installs) != 1 {
		t.Errorf("round-trip mismatch: %+v", s)
	}
}

func TestGetNotFound(t *testing.T) {
	m, _ := Load(filepath.Join(t.TempDir(), "x.json"))
	if _, err := m.Get("nope"); !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("expected ErrSkillNotFound, got %v", err)
	}
}

func TestFindInstall(t *testing.T) {
	s := &Skill{Installs: []Install{
		{Agent: "claude-code", Scope: "project"},
		{Agent: "claude-code", Scope: "global"},
	}}
	if i := s.FindInstall("claude-code", "global"); i != 1 {
		t.Errorf("FindInstall = %d, want 1", i)
	}
	if i := s.FindInstall("cursor", "project"); i != -1 {
		t.Errorf("FindInstall(absent) = %d, want -1", i)
	}
}

func TestSaveAtomicNoTempLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.json")
	m, _ := Load(path)
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "skills.json" {
			t.Errorf("leftover file after Save: %s", e.Name())
		}
	}
}
