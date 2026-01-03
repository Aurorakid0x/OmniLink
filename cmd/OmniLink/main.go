package main

import (
	https_server "OmniLink/api/http"
	"OmniLink/internal/config"
	"OmniLink/pkg/zlog"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 1. 加载配置
	conf := config.GetConfig()
	host := conf.MainConfig.Host
	port := conf.MainConfig.Port

	// 2. 启动 HTTP 服务
	go func() {
		addr := fmt.Sprintf("%s:%d", host, port)
		zlog.Info(fmt.Sprintf("服务器正在启动，监听地址: %s", addr))
		// 目前使用 HTTP 启动。如果需要 HTTPS，请配置证书并使用 GE.RunTLS
		if err := https_server.GE.Run(addr); err != nil {
			zlog.Fatal("服务器启动失败: " + err.Error())
			return
		}

		// 使用 HTTPS 启动
		// if err := https_server.GE.RunTLS(addr, "cert.pem", "key.pem"); err != nil {
		// 	zlog.Fatal("服务器启动失败: " + err.Error())
		// 	return
		// }
	}()

	// 3. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	<-quit

	zlog.Info("正在关闭服务器...")
	// 在此处添加资源释放逻辑（如关闭数据库连接、Redis 等）

	zlog.Info("服务器已关闭")
}
