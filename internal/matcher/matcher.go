// Package matcher 提供代理所需的 URL 模式匹配功能。
package matcher

import (
	"regexp"
	"strings"
)

var (
	expRelease = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+?)/([^/]+?)/(?:releases|archive)/.*$`)
	expBlob    = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+?)/([^/]+?)/(?:blob|raw)/.*$`)
	expGit     = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+?)/([^/]+?)/(?:info|git-).*$`)
	expRaw     = regexp.MustCompile(`^(?:https?://)?raw\.(?:githubusercontent|github)\.com/([^/]+?)/([^/]+?)/.+?/.+$`)
	expGist    = regexp.MustCompile(`^(?:https?://)?gist\.(?:githubusercontent|github)\.com/([^/]+?)/.+?/.+$`)

	// Hugging Face 匹配器
	expHFDatasetGit     = regexp.MustCompile(`^(?:https?://)?huggingface\.co/(datasets/[^/]+?)/([^/]+?)/(?:info|git-|resolve|raw|blob)/.*$`)
	expHFDatasetGitRoot = regexp.MustCompile(`^(?:https?://)?huggingface\.co/(datasets/[^/]+?)/(?:info|git-|resolve|raw|blob)/.*$`)
	expHFSpacesGit      = regexp.MustCompile(`^(?:https?://)?huggingface\.co/(spaces/[^/]+?)/([^/]+?)/(?:info|git-|resolve|raw|blob)/.*$`)
	expHFGit            = regexp.MustCompile(`^(?:https?://)?huggingface\.co/([^/]+?)/([^/]+?)/(?:info|git-|resolve|raw|blob)/.*$`)
	expHFGitRoot        = regexp.MustCompile(`^(?:https?://)?huggingface\.co/([^/]+?)/(?:info|git-|resolve|raw|blob)/.*$`)

	ghExps = []*regexp.Regexp{expRelease, expBlob, expGit, expRaw, expGist}
	hfExps = []*regexp.Regexp{
		expHFDatasetGit, expHFDatasetGitRoot, expHFSpacesGit, expHFGit, expHFGitRoot,
	}
)

// MatchURL 在匹配成功时返回捕获的分组（user[, repo]），否则返回 nil。
func MatchURL(u string) []string {
	if strings.Contains(u, "github") {
		for _, exp := range ghExps {
			if m := exp.FindStringSubmatch(u); m != nil {
				return m[1:]
			}
		}
	}
	if strings.Contains(u, "huggingface.co") {
		for _, exp := range hfExps {
			if m := exp.FindStringSubmatch(u); m != nil {
				return m[1:]
			}
		}
	}
	return nil
}

// IsBlob 报告 URL 是否为 GitHub 或 Hugging Face 的 blob (浏览器预览) URL。
func IsBlob(u string) bool {
	return expBlob.MatchString(u) || (strings.Contains(u, "/blob/") && IsHF(u))
}

// IsHF 报告 URL 是否为 Hugging Face URL。
func IsHF(u string) bool {
	return strings.Contains(u, "huggingface.co")
}
