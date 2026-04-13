package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	version    = "v0.1.0"
)

func main() {
	flag.Parse()

	// 打印版本信息
	fmt.Printf("GCS-Distill Server %s\n", version)
	fmt.Printf("加载配置文件: %s\n", *configPath)

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Initialize(&cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("GCS-Distill Server 启动中...")
	logger.Infof("配置: %+v", cfg.Server)

	// TODO: 初始化数据库连接
	// TODO: 初始化 Redis 连接
	// TODO: 初始化 gRPC 客户端
	// TODO: 启动 HTTP 服务器
	// TODO: 启动 gRPC 服务器

	logger.Info("服务器启动成功")

	// 等待信号
	waitForSignal()

	logger.Info("GCS-Distill Server 关闭中...")
	// TODO: 优雅关闭
}

// waitForSignal 等待系统信号
func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
