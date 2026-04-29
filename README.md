# gh-proxy (Go)

GitHub 和 Hugging Face 加速代理的 Go 重写版本。支持 Git Clone、Release、Blob 以及大文件下载加速。

## 特点

- **双平台支持** — 同时支持 GitHub 和 Hugging Face (模型、数据集、Spaces)
- **零依赖** — 仅使用 Go 标准库，编译后的二进制文件约 5 MB
- **低内存** — 采用流式转发和 `sync.Pool` 缓冲区复用，空闲内存占用仅 5–10 MB
- **多平台部署** — 支持 Docker、二进制运行以及 **Cloudflare Workers**
- **行为对齐** — 与原项目一致的白名单/黑名单/穿透名单语义、jsDelivr 跳转及 Location 递归

## 路由说明

| 路由 | 说明 |
| --- | --- |
| `GET /` | 首页 |
| `GET /?q=<url>` | 302 跳转到 `/<url>` |
| `GET /healthz` | 健康检查，返回 `ok` |
| `* /<url>` | 代理 GitHub 或 Hugging Face 资源 |

## 快速开始

### 1. 使用 Docker
```bash
docker run -d --name gh-proxy -p 8080:8080 --restart always cololi/gh-proxy:latest
```

### 2. 本地编译运行
```bash
make run
```

### 3. Cloudflare Workers
请查看 [deployments/README.md](deployments/README.md) 获取一键部署脚本。

## 使用示例

### GitHub 加速
```bash
# Git 克隆
git clone https://你的代理域名/https://github.com/user/repo

# 下载 Release 文件
wget https://你的代理域名/https://github.com/user/repo/releases/download/v1.0/file.zip
```

### Hugging Face 加速
```bash
# Git 克隆模型
git clone https://你的代理域名/https://huggingface.co/gpt2

# 下载模型文件 (支持 LFS 重定向加速)
wget https://你的代理域名/https://huggingface.co/gpt2/resolve/main/config.json

# Git 克隆数据集
git clone https://你的代理域名/https://huggingface.co/datasets/glue
```

## 配置说明 (环境变量)

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN` | `:8080` | 监听地址 |
| `ASSET_URL` | `https://hunshcn.github.io/gh-proxy` | 首页 HTML / favicon 来源 |
| `JSDELIVR` | `false` | GitHub 的 `blob`/`raw` 是否跳转到 jsDelivr CDN |
| `SIZE_LIMIT` | `1072668082176` | 文件大小限制，超出则 302 跳转到原始地址 |
| `WHITE_LIST` | 空 | 白名单 (多行字符串) |
| `BLACK_LIST` | 空 | 黑名单 (多行字符串) |
| `PASS_LIST` | 空 | 穿透名单 (直接 302，不经过代理流转) |

## 项目结构

```
.
├── cmd/                # 程序入口
├── deployments/        # 部署配置 (Docker, Cloudflare Workers)
├── internal/           # 内部逻辑实现
│   ├── config/         # 配置解析
│   ├── matcher/        # URL 匹配与识别
│   └── proxy/          # 代理请求处理
└── Makefile            # 构建与自动化脚本
```

## 许可证

[MIT](LICENSE)
