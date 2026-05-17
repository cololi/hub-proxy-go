package matcher

import (
	"reflect"
	"testing"
)

func TestMatchHFURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []string
	}{
		{"hf model git info", "https://huggingface.co/gpt2/info/refs", []string{"gpt2"}},
		{"hf user model git info", "https://huggingface.co/user/repo/info/refs", []string{"user", "repo"}},
		{"hf dataset git info", "https://huggingface.co/datasets/user/repo/info/refs", []string{"datasets/user", "repo"}},
		{"hf dataset root git info", "https://huggingface.co/datasets/glue/info/refs", []string{"datasets/glue"}},
		{"hf model root git upload pack", "https://huggingface.co/gpt2/git-upload-pack", []string{"gpt2"}},
		{"hf space git info", "https://huggingface.co/spaces/user/repo/info/refs", []string{"spaces/user", "repo"}},
		{"hf model git upload pack", "https://huggingface.co/user/repo/git-upload-pack", []string{"user", "repo"}},
		{"hf model resolve", "https://huggingface.co/user/repo/resolve/main/config.json", []string{"user", "repo"}},
		{"hf model blob", "https://huggingface.co/user/repo/blob/main/README.md", []string{"user", "repo"}},
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
