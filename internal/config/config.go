// Package config 处理从环境变量加载的运行配置。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 存储从环境变量加载的所有配置。
type Config struct {
	Listen          string
	AssetURL        string
	JSDelivr        bool
	SizeLimit       int64
	BufferSize      int
	UpstreamTimeout time.Duration
	ShutdownTimeout time.Duration

	WhiteList []Rule
	BlackList []Rule
	PassList  []Rule
}

// Rule 是一个访问控制条目。Repo == "" 表示“匹配该用户下的任何仓库”。
// User == "*" 表示“匹配任何用户的特定仓库”，此时 Repo 必须设置。
type Rule struct {
	User string
	Repo string
}

// Load 从环境变量读取配置，并设置默认值。
func Load() *Config {
	return &Config{
		Listen:          env("LISTEN", ":8080"),
		AssetURL:        env("ASSET_URL", "https://hunshcn.github.io/gh-proxy"),
		JSDelivr:        envBool("JSDELIVR", false),
		SizeLimit:       envInt64("SIZE_LIMIT", 1072668082176),
		BufferSize:      int(envInt64("BUFFER_SIZE", 32*1024)),
		UpstreamTimeout: envDuration("UPSTREAM_TIMEOUT", 30*time.Second),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		WhiteList:       parseRules(envOrFile("WHITE_LIST", "WHITE_LIST_FILE")),
		BlackList:       parseRules(envOrFile("BLACK_LIST", "BLACK_LIST_FILE")),
		PassList:        parseRules(envOrFile("PASS_LIST", "PASS_LIST_FILE")),
	}
}

// Match 报告规则是否匹配给定的 URL 分组。
// 匹配语义：
//
//	"user1"        -> 匹配 user1 的所有仓库
//	"user1/repo1"  -> 仅匹配 user1/repo1
//	"*/repo1"      -> 匹配任何用户的 repo1 仓库
func (r Rule) Match(groups []string) bool {
	if r.User == "*" {
		return r.Repo != "" && len(groups) >= 2 && groups[1] == r.Repo
	}
	if len(groups) == 0 || groups[0] != r.User {
		return false
	}
	if r.Repo == "" {
		return true
	}
	return len(groups) >= 2 && groups[1] == r.Repo
}

// AnyMatch 如果规则列表中的任何一条规则匹配，则返回 true。
func AnyMatch(rules []Rule, groups []string) bool {
	for _, r := range rules {
		if r.Match(groups) {
			return true
		}
	}
	return false
}

// parseRules 解析多行规则列表。
func parseRules(s string) []Rule {
	if s == "" {
		return nil
	}
	var rules []Rule
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "/", 2)
		rule := Rule{User: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			rule.Repo = strings.TrimSpace(parts[1])
		}
		rules = append(rules, rule)
	}
	return rules
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// envOrFile 返回环境变量的值，如果未设置则从指定的文件中读取。
func envOrFile(envKey, fileKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if path := os.Getenv(fileKey); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return ""
}
