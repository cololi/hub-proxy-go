# Hub-Proxy-Go 项目指南

Hub-Proxy-Go 是一个高性能的 GitHub 和 Hugging Face 透明代理服务器，旨在加速 Git 克隆、Release 下载和大文件获取。

## 项目概览

- **核心功能**: 转发请求至 GitHub 和 Hugging Face，支持自动将 GitHub `blob` 链接转换为 `raw` 链接。
- **技术栈**: Go 1.22+, 基于流式转发 (Streaming) 减少内存占用，利用 `sync.Pool` 复用字节缓冲区。
- **性能**: 实测典型负载下内存占用 (RSS) 仅约 10MB。

## 构建与运行

项目使用 `Makefile` 管理常用任务：

- **本地构建**: `make build`
- **运行程序**: `make run`
- **发布构建**: `make dist`

## 开发规范

- **编程语言**: Go (Golang)，遵循 Google Go 编码规范。
- **文档语言**: **所有代码注释、README 及文档必须使用中文**。
- **Git 规范**: 
    - 远程仓库: `git@github.com:cololi/hub-proxy-go.git`
    - `.gitignore` 和 `.dockerignore` 不提交至远程仓库。
- **CI/CD**:
    - 标签推送 (`v*.*.*`) 会触发纯中文说明的二进制文件发布。

## 配置参考

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LISTEN` | `:8080` | 服务监听地址 |
| `SIZE_LIMIT` | `1072668082176` | 最大代理文件大小（字节） |
| `UPSTREAM_TIMEOUT` | `30s` | 请求上游的超时时间 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅停机的超时时间 |
