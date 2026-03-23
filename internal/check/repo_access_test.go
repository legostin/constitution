package check

import "testing"

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"git@github.com:acme-corp/myrepo.git", "github.com/acme-corp/myrepo"},
		{"https://github.com/acme-corp/myrepo.git", "github.com/acme-corp/myrepo"},
		{"https://github.com/acme-corp/myrepo", "github.com/acme-corp/myrepo"},
		{"http://github.com/acme-corp/myrepo.git", "github.com/acme-corp/myrepo"},
		{"git@gitlab.com:team/project.git", "gitlab.com/team/project"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeRemoteURL(tt.input)
			if got != tt.want {
				t.Errorf("normalizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchRepoPattern(t *testing.T) {
	tests := []struct {
		pattern string
		repo    string
		match   bool
	}{
		{"github.com/acme-corp/*", "github.com/acme-corp/myrepo", true},
		{"github.com/acme-corp/*", "github.com/other-org/myrepo", false},
		{"github.com/acme-corp/myrepo", "github.com/acme-corp/myrepo", true},
		{"github.com/acme-corp/myrepo", "github.com/acme-corp/other", false},
		{"gitlab.com/*/*", "gitlab.com/team/project", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.repo, func(t *testing.T) {
			got := matchRepoPattern(tt.pattern, tt.repo)
			if got != tt.match {
				t.Errorf("matchRepoPattern(%q, %q) = %v, want %v", tt.pattern, tt.repo, got, tt.match)
			}
		})
	}
}
