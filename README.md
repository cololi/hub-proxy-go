# Hub-Proxy-Go

GitHub 和 Hugging Face 加速代理。支持 Git Clone、Release、Blob 以及大文件下载加速。

## 特点

- **双平台支持** — 同时支持 GitHub 和 Hugging Face (模型、数据集、Spaces)
- **高性能** — 采用流式转发和 `sync.Pool` 缓冲区复用，极低内存占用
- **简单部署** — 支持 Docker (GHCR)、二进制运行以及 systemd
- **自动转换** — 自动将 GitHub 的 `blob` 预览链接转换为 `raw` 直链下载

## 快速开始

### 1. 使用 Docker (GHCR)
```bash
docker run -d --name hub-proxy-go -p 8080:8080 --restart always ghcr.io/${GITHUB_USER}/hub-proxy-go:master
```
> 请将 `${GITHUB_USER}` 替换为您的 GitHub 用户名。

### 2. 使用 systemd (Linux 推荐)

1. 下载或编译 `hub-proxy-go` 二进制文件：
   ```bash
   make build
   sudo cp hub-proxy-go /usr/local/bin/
   ```
2. 创建服务文件 `/etc/systemd/system/hub-proxy-go.service`：
   ```ini
   [Unit]
   Description=Hub-Proxy-Go Service
   After=network.target

   [Service]
   ExecStart=/usr/local/bin/hub-proxy-go
   Restart=always
   User=root

   [Install]
   WantedBy=multi-user.target
   ```
3. 启动并启用服务：
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now hub-proxy-go
   ```

### 3. 本地编译运行
```bash
make run
```

## 配置说明 (环境变量)

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN` | `:8080` | 监听地址 |
| `ASSET_URL` | `https://hunshcn.github.io/gh-proxy` | 首页 HTML / favicon 来源 |
| `SIZE_LIMIT` | `1072668082176` | 文件大小限制，超出则 302 跳转到原始地址 |

## 使用示例

### GitHub 加速
```bash
# Git 克隆
git clone https://你的域名/https://github.com/user/repo
```

### Hugging Face 加速
```bash
# Git 克隆模型
git clone https://你的域名/https://huggingface.co/gpt2
```
