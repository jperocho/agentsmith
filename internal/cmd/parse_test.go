package cmd

import "testing"

func TestParseRepo(t *testing.T) {
	cases := []struct {
		in      string
		wantURL string
		wantRef string
		wantNm  string
	}{
		{"owner/name", "https://github.com/owner/name", "", "name"},
		{"owner/name@v1.2.0", "https://github.com/owner/name", "v1.2.0", "name"},
		{"https://github.com/o/repo", "https://github.com/o/repo", "", "repo"},
		{"https://github.com/o/repo.git", "https://github.com/o/repo.git", "", "repo"},
		{"https://gitlab.com/o/repo@main", "https://gitlab.com/o/repo", "main", "repo"},
		{"git@github.com:o/repo.git", "git@github.com:o/repo.git", "", "repo"},
		{"git@github.com:o/repo.git@v2", "git@github.com:o/repo.git", "v2", "repo"},
		{"file:///tmp/local/myskill", "file:///tmp/local/myskill", "", "myskill"},
	}
	for _, c := range cases {
		got, err := parseRepo(c.in)
		if err != nil {
			t.Errorf("parseRepo(%q) error: %v", c.in, err)
			continue
		}
		if got.URL != c.wantURL || got.Ref != c.wantRef || got.Name != c.wantNm {
			t.Errorf("parseRepo(%q) = {URL:%q Ref:%q Name:%q}, want {URL:%q Ref:%q Name:%q}",
				c.in, got.URL, got.Ref, got.Name, c.wantURL, c.wantRef, c.wantNm)
		}
	}
}

func TestParseRepoRejects(t *testing.T) {
	bad := []string{"", "not a url", "ftp://x/y", "../escape"}
	for _, in := range bad {
		if _, err := parseRepo(in); err == nil {
			t.Errorf("parseRepo(%q) = nil error, want rejection", in)
		}
	}
}

func TestDeriveNameRejectsBadChars(t *testing.T) {
	if n := deriveName("https://h/o/bad name"); n != "" {
		t.Errorf("deriveName allowed bad chars: %q", n)
	}
}
