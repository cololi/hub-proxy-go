package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cololi/Hub-Proxy-Go/internal/config"
	"github.com/cololi/Hub-Proxy-Go/internal/proxy"
)

func main() {
	cfg := config.Load()
	p := proxy.New(cfg)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           p,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 在后台启动服务器
	go func() {
		log.Printf("Hub-Proxy-Go 正在监听 %s (文件大小限制=%d 字节)",
			cfg.Listen, cfg.SizeLimit)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务器错误: %v", err)
		}
	}()

	// 优雅停机处理
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("停机过程中出现错误: %v", err)
	}
	log.Println("已退出")
}
