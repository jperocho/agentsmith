package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryHasExpectedAgents(t *testing.T) {
	want := []string{"claude-code", "cursor", "codex", "hermes"}
	for _, name := range want {
		if _, ok := Get(name); !ok {
			t.Errorf("adapter %q missing from registry", name)
		}
	}
	if _, ok := Get("nonexistent"); ok {
		t.Errorf("Get(nonexistent) reported ok")
	}
}

func TestDirScopes(t *testing.T) {
	ad, _ := Get("claude-code")
	cwd := "/proj"
	proj, err := ad.Dir(Project, cwd, "myskill")
	if err != nil {
		t.Fatal(err)
	}
	if proj != filepath.Join(cwd, ".claude", "skills", "myskill") {
		t.Errorf("project dir = %q", proj)
	}
	glob, err := ad.Dir(Global, cwd, "myskill")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(glob, filepath.Join(".claude", "skills", "myskill")) {
		t.Errorf("global dir = %q", glob)
	}
}

func TestAllowedRootsIncludeBothScopes(t *testing.T) {
	ad, _ := Get("cursor")
	roots := ad.AllowedRoots("/proj")
	if len(roots) < 2 {
		t.Fatalf("expected >=2 roots, got %v", roots)
	}
	foundProject := false
	for _, r := range roots {
		if strings.HasPrefix(r, "/proj") {
			foundProject = true
		}
	}
	if !foundProject {
		t.Errorf("project root missing from %v", roots)
	}
}
