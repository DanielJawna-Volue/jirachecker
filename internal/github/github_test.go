package github

import "testing"

func TestParseRepo(t *testing.T) {
	for _, v := range []string{"owner/repo", "https://github.com/owner/repo", "https://github.com/owner/repo.git",
		"git@github.com:owner/repo.git", " owner/repo/ "} {
		got, err := ParseRepo(v)
		if err != nil || got != "owner/repo" {
			t.Errorf("ParseRepo(%q) = %q, %v; want owner/repo", v, got, err)
		}
	}
	for _, v := range []string{"repo", "a/b/c", ""} {
		if _, err := ParseRepo(v); err == nil {
			t.Errorf("ParseRepo(%q) succeeded, want error", v)
		}
	}
}
