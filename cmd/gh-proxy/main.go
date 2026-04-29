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

	"github.com/cololi/gh-proxy/internal/config"
	"github.com/cololi/gh-proxy/internal/proxy"
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
		log.Printf("gh-proxy 正在监听 %s (jsDelivr=%v, 文件大小限制=%d 字节)",
			cfg.Listen, cfg.JSDelivr, cfg.SizeLimit)
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
