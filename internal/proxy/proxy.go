// Package proxy 实现 GitHub 和 Hugging Face 资源的 HTTP 反向代理。
package proxy

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cololi/Hub-Proxy-Go/internal/config"
	"github.com/cololi/Hub-Proxy-Go/internal/matcher"
)

const maxRedirectDepth = 5

// Proxy 实现 http.Handler。它拥有 HTTP 客户端、缓冲区池以及缓存的静态资源。
type Proxy struct {
	cfg     *config.Config
	client  *http.Client
	bufPool sync.Pool

	indexHTML []byte
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
		indexHTML: []byte(defaultIndexHTML),
	}
	p.bufPool.New = func() any {
		buf := make([]byte, cfg.BufferSize)
		return &buf
	}

	return p
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
		// 由于删除了 ASSET_URL，不再动态获取 favicon。
		http.NotFound(w, r)
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

	if matcher.MatchURL(target) == nil {
		http.Error(w, "无效的输入 URL", http.StatusForbidden)
		return
	}

	// 自动将 blob 转换为 raw
	if matcher.IsBlob(target) {
		target = strings.Replace(target, "/blob/", "/raw/", 1)
	}

	p.streamProxy(w, r.Context(), target, r.Method, r.Body, r.Header, 0)
}

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
		// 跳过 Host 和逐跳请求头
		if strings.EqualFold(k, "Host") || isHopByHop(k) {
			continue
		}
		req.Header[k] = vs
	}

	// 如果 User-Agent 为空，设置默认的以避免部分上游拦截
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Hub-Proxy-Go/1.0")
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
			// 处理相对路径重定向
			if !strings.HasPrefix(loc, "http") {
				base, _ := url.Parse(target)
				locURL, _ := base.Parse(loc)
				loc = locURL.String()
			}

			if matcher.MatchURL(loc) != nil {
				w.Header().Set("Location", "/"+loc)
				w.WriteHeader(resp.StatusCode)
				return
			} else {
				drainAndClose(resp.Body)
				// 跨域重定向时清理敏感头
				newHeaders := make(http.Header)
				for k, v := range headers {
					if !isSensitiveHeader(k) {
						newHeaders[k] = v
					}
				}
				p.streamProxy(w, ctx, loc, http.MethodGet, http.NoBody, newHeaders, depth+1)
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

func normalizeURL(u string) string {
	if strings.HasPrefix(u, "https:/") && !strings.HasPrefix(u, "https://") {
		return "https://" + u[7:]
	}
	if strings.HasPrefix(u, "http:/") && !strings.HasPrefix(u, "http://") {
		return "http://" + u[6:]
	}
	if !strings.HasPrefix(u, "http") {
		return "https://" + u
	}
	return u
}

func isSensitiveHeader(h string) bool {
	// h 预期已经是规范化的（来自 http.Header）
	return h == "Authorization" || h == "Cookie" || h == "Set-Cookie"
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
	_, ok := hopByHopHeaders[h]
	return ok
}

const defaultIndexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Hub-Proxy-Go</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; max-width: 720px; margin: 60px auto; padding: 0 24px; color: #222; line-height: 1.5; }
        h1 { margin-bottom: 8px; font-weight: 700; }
        p { color: #666; margin-top: 0; }
        form { display: flex; gap: 8px; margin: 24px 0; }
        input { flex: 1; padding: 12px 14px; font-size: 16px; border: 1px solid #ccc; border-radius: 6px; outline: none; }
        input:focus { border-color: #0366d6; box-shadow: 0 0 0 3px rgba(3,102,214,0.1); }
        button { padding: 12px 18px; font-size: 16px; border: 0; background: #0366d6; color: #fff; border-radius: 6px; cursor: pointer; font-weight: 600; }
        button:hover { background: #0255b3; }
        h3 { margin-top: 32px; font-size: 18px; }
        pre { background: #f6f8fa; padding: 12px 16px; border-radius: 6px; font-size: 13px; overflow-x: auto; color: #444; border: 1px solid #eaecef; }
    </style>
</head>
<body>
    <h1>Hub-Proxy-Go</h1>
    <p>GitHub 和 Hugging Face 加速代理。在下方输入 URL 即可开始。</p>
    <form method="get" action="/">
        <input name="q" placeholder="https://github.com/user/repo/..." autofocus required>
        <button type="submit">前往</button>
    </form>
    <h3>示例</h3>
    <pre>https://github.com/user/repo/releases/download/v1.0/file.zip
https://github.com/user/repo/archive/refs/heads/main.zip
https://raw.githubusercontent.com/user/repo/main/README.md
https://huggingface.co/gpt2/resolve/main/config.json</pre>
</body>
</html>`
