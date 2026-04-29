package matcher

import (
	"reflect"
	"testing"
)

func TestMatchURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []string
	}{
		{"release", "https://github.com/user/repo/releases/download/v1/file.zip", []string{"user", "repo"}},
		{"archive", "https://github.com/user/repo/archive/main.zip", []string{"user", "repo"}},
		{"blob", "https://github.com/user/repo/blob/main/README.md", []string{"user", "repo"}},
		{"raw on github.com", "https://github.com/user/repo/raw/main/file.txt", []string{"user", "repo"}},
		{"git info", "https://github.com/user/repo/info/refs", []string{"user", "repo"}},
		{"git-upload-pack", "https://github.com/user/repo/git-upload-pack", []string{"user", "repo"}},
		{"raw subdomain", "https://raw.githubusercontent.com/user/repo/main/file.txt", []string{"user", "repo"}},
		{"raw github short", "https://raw.github.com/user/repo/main/file.txt", []string{"user", "repo"}},
		{"gist", "https://gist.githubusercontent.com/user/abcdef/raw/file.txt", []string{"user"}},
		{"no scheme", "github.com/user/repo/releases/v1/x", []string{"user", "repo"}},
		{"http scheme", "http://github.com/user/repo/blob/main/x", []string{"user", "repo"}},
		{"non-github", "https://example.com/user/repo/blob/main/x", nil},
		{"github root", "https://github.com/user/repo", nil},
		{"github tree", "https://github.com/user/repo/tree/main", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchURL(tt.url)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
