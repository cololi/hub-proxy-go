// Package proxy 实现 GitHub 和 Hugging Face 资源的 HTTP 反向代理。
package proxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cololi/gh-proxy/internal/config"
	"github.com/cololi/gh-proxy/internal/matcher"
)

const maxRedirectDepth = 5

// Proxy 实现 http.Handler。它拥有 HTTP 客户端、缓冲区池以及缓存的静态资源。
type Proxy struct {
	cfg     *config.Config
	client  *http.Client
	bufPool sync.Pool

	indexHTML  []byte
	faviconICO []byte
}

// New 根据给定配置创建一个新的 Proxy。
func New(cfg *config.Config) *Proxy {
	dialer := &net.Dialer{
		Timeout:   cfg.UpstreamTimeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   cfg.UpstreamTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
	}

	p := &Proxy{
		cfg: cfg,
		client: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	p.bufPool.New = func() any {
		buf := make([]byte, cfg.BufferSize)
		return &buf
	}

	p.loadAssets()
	return p
}

// loadAssets 在启动时从 AssetURL 获取首页 HTML 和 favicon。
// 失败时将回退到内置的极简首页。
func (p *Proxy) loadAssets() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if data, err := fetchSmall(ctx, p.cfg.AssetURL); err == nil {
		p.indexHTML = data
	} else {
		log.Printf("警告: 无法从 %s 获取首页: %v (使用内置备用页面)", p.cfg.AssetURL, err)
		p.indexHTML = []byte(defaultIndexHTML)
	}

	if data, err := fetchSmall(ctx, p.cfg.AssetURL+"/favicon.ico"); err == nil {
		p.faviconICO = data
	}
}

func fetchSmall(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
}

// ServeHTTP 实现 http.Handler 接口。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	switch path {
	case "":
		if q := r.URL.Query().Get("q"); q != "" {
			http.Redirect(w, r, "/"+q, http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write(p.indexHTML)
		return
	case "favicon.ico":
		if len(p.faviconICO) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/vnd.microsoft.icon")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(p.faviconICO)
		return
	case "healthz":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	p.handleProxy(w, r, path)
}

func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request, path string) {
	target := path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	target = normalizeURL(target)

	groups := matcher.MatchURL(target)
	if groups == nil {
		http.Error(w, "无效的输入", http.StatusForbidden)
		return
	}

	if len(p.cfg.WhiteList) > 0 && !config.AnyMatch(p.cfg.WhiteList, groups) {
		http.Error(w, "白名单限制访问", http.StatusForbidden)
		return
	}
	if config.AnyMatch(p.cfg.BlackList, groups) {
		http.Error(w, "黑名单禁止访问", http.StatusForbidden)
		return
	}
	passBy := config.AnyMatch(p.cfg.PassList, groups)

	// jsDelivr 重写 (仅针对 GitHub)
	if (p.cfg.JSDelivr || passBy) && !matcher.IsHF(target) {
		if matcher.IsBlob(target) {
			jd := strings.Replace(target, "/blob/", "@", 1)
			jd = strings.Replace(jd, "github.com", "cdn.jsdelivr.net/gh", 1)
			http.Redirect(w, r, jd, http.StatusFound)
			return
		}
		if matcher.IsRaw(target) {
			jd := matcher.RawRewrite().ReplaceAllString(target, "${1}@${2}")
			swapped := strings.Replace(jd, "raw.githubusercontent.com", "cdn.jsdelivr.net/gh", 1)
			if swapped == jd {
				swapped = strings.Replace(jd, "raw.github.com", "cdn.jsdelivr.net/gh", 1)
			}
			http.Redirect(w, r, swapped, http.StatusFound)
			return
		}
	}

	// 自动将 blob 转换为 raw
	if matcher.IsBlob(target) {
		target = strings.Replace(target, "/blob/", "/raw/", 1)
	}

	if passBy {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	p.streamProxy(w, r.Context(), target, r.Method, r.Body, r.Header, 0)
}

// streamProxy 从上游获取目标资源并将其流式传输回客户端。
// 对于带有 Location 响应头的 3xx 响应，它会重写该头以便通过代理重定向，
// 或者在服务器端跟随重定向（取决于 Location 是否匹配已知的模式）。
func (p *Proxy) streamProxy(
	w http.ResponseWriter,
	ctx context.Context,
	target, method string,
	body io.Reader,
	headers http.Header,
	depth int,
) {
	if depth > maxRedirectDepth {
		http.Error(w, "重定向次数过多", http.StatusBadGateway)
		return
	}

	parsed, err := url.Parse(target)
	if err != nil {
		http.Error(w, "无效的 URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), body)
	if err != nil {
		http.Error(w, "服务器错误: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for k, vs := range headers {
		if strings.EqualFold(k, "Host") || isHopByHop(k) {
			continue
		}
		req.Header[k] = vs
	}

	resp, err := p.client.Do(req)
	if err != nil {
		http.Error(w, "上游服务器错误: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if size, err := strconv.ParseInt(cl, 10, 64); err == nil && size > p.cfg.SizeLimit {
			drainAndClose(resp.Body)
			w.Header().Set("Location", target)
			w.WriteHeader(http.StatusFound)
			return
		}
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); loc != "" {
			if matcher.MatchURL(loc) != nil {
				resp.Header.Set("Location", "/"+loc)
			} else {
				drainAndClose(resp.Body)
				p.streamProxy(w, ctx, loc, http.MethodGet, http.NoBody, headers, depth+1)
				return
			}
		}
	}

	dst := w.Header()
	for k, vs := range resp.Header {
		if isHopByHop(k) {
			continue
		}
		dst[k] = vs
	}
	w.WriteHeader(resp.StatusCode)

	bufPtr := p.bufPool.Get().(*[]byte)
	defer p.bufPool.Put(bufPtr)

	if _, err := io.CopyBuffer(w, resp.Body, *bufPtr); err != nil {
		log.Printf("代理: %s 的流传输错误: %v", target, err)
	}
}

func drainAndClose(r io.ReadCloser) {
	_, _ = io.CopyN(io.Discard, r, 8*1024)
	_ = r.Close()
}

// normalizeURL 确保 URL 是可用的绝对路径。
func normalizeURL(u string) string {
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	upper := 9
	if len(u) < upper {
		upper = len(u)
	}
	if !strings.Contains(u[:upper], "://") {
		u = strings.Replace(u, "s:/", "s://", 1)
	}
	return u
}

var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Proxy-Connection":    {},
	"Keep-Alive":          {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
}

func isHopByHop(h string) bool {
	_, ok := hopByHopHeaders[http.CanonicalHeaderKey(h)]
	return ok
}

const defaultIndexHTML = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>gh-proxy</title>
<style>
  body { font-family: system-ui, -apple-system, sans-serif; max-width: 720px;
         margin: 60px auto; padding: 0 24px; color: #222; }
  h1 { margin-bottom: 8px; }
  p { color: #666; margin-top: 0; }
  form { display: flex; gap: 8px; margin: 24px 0; }
  input { flex: 1; padding: 12px 14px; font-size: 16px;
          border: 1px solid #ccc; border-radius: 6px; }
  button { padding: 12px 18px; font-size: 16px; border: 0;
           background: #0366d6; color: #fff; border-radius: 6px; cursor: pointer; }
  pre { background: #f6f8fa; padding: 12px 16px; border-radius: 6px;
        font-size: 13px; overflow-x: auto; }
</style></head>
<body>
  <h1>GitHub / Hugging Face 代理</h1>
  <p>在下方输入 GitHub 或 Hugging Face 的 URL，然后按回车确认。</p>
  <form method="get" action="/">
    <input name="q" placeholder="https://github.com/user/repo/..." autofocus required>
    <button type="submit">前往</button>
  </form>
  <h3>示例</h3>
  <pre>https://github.com/user/repo/releases/download/v1.0/file.zip
https://github.com/user/repo/archive/refs/heads/main.zip
https://raw.githubusercontent.com/user/repo/main/README.md
https://huggingface.co/gpt2/resolve/main/config.json</pre>
</body></html>`
