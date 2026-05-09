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
    <title>Hub-Proxy-Go | 高性能 GitHub/HuggingFace 加速</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; max-width: 800px; margin: 80px auto; padding: 0 20px; color: #24292e; line-height: 1.6; background-color: #f6f8fa; }
        .container { background: #fff; padding: 40px; border-radius: 8px; box-shadow: 0 1px 3px rgba(27,31,35,0.12), 0 1px 2px rgba(27,31,35,0.24); }
        h1 { margin-top: 0; font-size: 28px; font-weight: 600; color: #0366d6; }
        p { color: #586069; margin-bottom: 30px; }
        .input-group { display: flex; gap: 10px; margin-bottom: 30px; }
        input { flex: 1; padding: 12px 16px; font-size: 16px; border: 1px solid #d1d5da; border-radius: 6px; outline: none; transition: border-color 0.2s, box-shadow 0.2s; }
        input:focus { border-color: #0366d6; box-shadow: 0 0 0 3px rgba(3,102,214,0.3); }
        button { padding: 12px 24px; font-size: 16px; font-weight: 600; color: #fff; background-color: #2ea44f; border: 1px solid rgba(27,31,35,0.15); border-radius: 6px; cursor: pointer; transition: background-color 0.2s; }
        button:hover { background-color: #2c974b; }
        h2 { font-size: 18px; font-weight: 600; margin-top: 30px; border-bottom: 1px solid #eaecef; padding-bottom: 8px; }
        pre { background: #f6f8fa; padding: 16px; border-radius: 6px; font-size: 14px; overflow-x: auto; color: #444; border: 1px solid #dfe1e4; margin-top: 10px; }
        code { font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace; }
        ul { padding-left: 20px; color: #586069; }
        li { margin-bottom: 8px; }
        .footer { margin-top: 40px; text-align: center; font-size: 12px; color: #6a737d; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Hub-Proxy-Go</h1>
        <p>高性能透明代理，支持 GitHub (Releases, Archive, Blob, Raw) 和 Hugging Face 资源加速。</p>
        
        <form method="get" action="/">
            <div class="input-group">
                <input name="q" placeholder="输入 GitHub 或 Hugging Face 链接..." autofocus required>
                <button type="submit">立即加速</button>
            </div>
        </form>

        <h2>支持的格式示例</h2>
        <pre><code># GitHub Release / 源码包
https://github.com/user/repo/releases/download/v1.0/file.zip
https://github.com/user/repo/archive/refs/heads/main.zip

# GitHub 文件 (自动转换为 Raw)
https://github.com/user/repo/blob/main/README.md

# Hugging Face 模型/数据集
https://huggingface.co/gpt2/resolve/main/config.json
https://huggingface.co/datasets/user/data/resolve/main/file.csv

# Git 克隆加速
git clone {{.Origin}}/https://github.com/user/repo.git</code></pre>

        <h2>使用说明</h2>
        <ul>
            <li>直接在输入框粘贴链接并点击“立即加速”。</li>
            <li>对于 Git 克隆，在原链接前加上本站地址。</li>
            <li>本项目仅用于合规的学习与研究目的，请勿用于非法用途。</li>
        </ul>
    </div>
    <div class="footer">
        Powered by Hub-Proxy-Go
    </div>
    <script>
        // 自动替换 Origin 占位符
        document.body.innerHTML = document.body.innerHTML.replace(/{{.Origin}}/g, window.location.origin);
    </script>
</body>
</html>`
