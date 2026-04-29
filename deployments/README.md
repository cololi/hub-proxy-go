# 部署指南

本项目支持多种部署方式。

## 1. Cloudflare Workers 部署

Cloudflare Workers 是最推荐的部署方式，具有全球加速和自动处理 LFS 重定向的优势。

- **代码文件**: `deployments/cloudflare-worker.js`
- **部署步骤**:
  1. 登录 [Cloudflare 控制台](https://dash.cloudflare.com/)。
  2. 创建一个新的 Worker。
  3. 将 `deployments/cloudflare-worker.js` 中的内容粘贴到编辑器中。
  4. (可选) 在 设置 (Settings) -> 变量 (Variables) 中配置 `WHITE_LIST` 等环境变量。
  5. 绑定你的自定义域名以获得更稳定的访问。

## 2. Docker 部署

适合在自己的服务器或容器云平台上运行。

- **构建镜像**:
  ```bash
  docker build -t gh-proxy:go .
  ```
- **运行容器**:
  ```bash
  docker run -d --name gh-proxy -p 8080:8080 -e JSDELIVR=true gh-proxy:go
  ```

## 3. 直接运行 (Go 二进制)

适合直接在 Linux/macOS/Windows 服务器上作为系统服务运行。

- **编译**:
  ```bash
  go build -ldflags="-s -w" -trimpath -o gh-proxy ./cmd/gh-proxy
  ```
- **运行**:
  ```bash
  LISTEN=:8080 ./gh-proxy
  ```

## 环境变量说明

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN` | `:8080` | 监听地址 (仅限 Go 版本) |
| `ASSET_URL` | `https://hunshcn.github.io/gh-proxy` | 首页 HTML 来源 |
| `JSDELIVR` | `false` | 是否开启 jsDelivr 加速 |
| `WHITE_LIST` | 空 | 白名单 (多行) |
| `BLACK_LIST` | 空 | 黑名单 (多行) |
| `PASS_LIST` | 空 | 穿透名单 (多行) |
