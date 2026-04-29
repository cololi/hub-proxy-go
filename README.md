# Hub-Proxy-Go

GitHub 和 Hugging Face 加速代理。支持 Git Clone、Release、Blob 以及大文件下载加速。

## 特点

- **双平台支持** — 同时支持 GitHub 和 Hugging Face (模型、数据集、Spaces)
- **简单部署** — 支持 Docker、二进制运行以及 systemd (用户态)
- **自动转换** — 自动将 GitHub 的 `blob` 预览链接转换为 `raw` 直链下载

## 性能表现

本项目采用 **流式转发 (Streaming)** 技术，配合 Go 语言的 `sync.Pool` 缓冲区复用，实现了极低的性能开销：
- **内存占用**：典型负载下 RSS 占用仅约 **10MB**。
- **并发能力**：通过异步 I/O 能够处理大量并发下载请求。
- **低延迟**：不进行磁盘缓存，数据在内存中直接透传，响应速度极快。

## 快速开始

### 1. 使用 Docker

镜像发布在 GHCR 和 Docker Hub：

**Docker Hub:**
```bash
docker run -d --name hub-proxy-go -p 8080:8080 --restart always cololi/hub-proxy-go:latest
```

**GHCR:**
```bash
docker run -d --name hub-proxy-go -p 8080:8080 --restart always ghcr.io/cololi/hub-proxy-go:latest
```

### 2. 使用 systemd (Linux 推荐)

<details>
<summary><b>一键安装脚本 (推荐 - 用户态)</b></summary>

```bash
curl -sSL https://raw.githubusercontent.com/cololi/Hub-Proxy-Go/master/scripts/install.sh | bash
```
</details>

<details>
<summary><b>手动安装 (用户态)</b></summary>

1. 下载或编译 `hub-proxy-go` 二进制文件至用户目录：
   ```bash
   make build
   mkdir -p ~/.local/bin
   cp hub-proxy-go ~/.local/bin/
   ```
2. 创建用户服务文件 `~/.config/systemd/user/hub-proxy-go.service`：
   ```ini
   [Unit]
   Description=Hub-Proxy-Go Service
   After=network.target

   [Service]
   ExecStart=%h/.local/bin/hub-proxy-go
   Restart=always
   Environment=LISTEN=:8080

   [Install]
   WantedBy=default.target
   ```
3. 启动并启用服务：
   ```bash
   systemctl --user daemon-reload
   systemctl --user enable --now hub-proxy-go
   ```

**查看日志:**
```bash
journalctl --user -u hub-proxy-go -f
```

**持久化运行:**
执行以下命令确保用户注销后服务继续运行：
```bash
sudo loginctl enable-linger $(whoami)
```
</details>

### 3. 本地编译运行

<details>
<summary><b>展开查看本地运行步骤</b></summary>

```bash
make run
```
</details>

## 配置说明 (环境变量)

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN` | `:8080` | 监听地址 |
| `SIZE_LIMIT` | `1072668082176` | 文件大小限制，超出则 302 跳转到原始地址 |
| `UPSTREAM_TIMEOUT` | `30s` | 上游连接超时时间 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅停机超时时间 |

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
