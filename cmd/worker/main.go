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
	nodeName   = flag.String("node-name", "", "Worker 节点名称 (必填)")
	version    = "v0.1.0"
)

func main() {
	flag.Parse()

	// 验证参数
	if *nodeName == "" {
		fmt.Fprintf(os.Stderr, "错误: 必须指定 --node-name 参数\n")
		flag.Usage()
		os.Exit(1)
	}

	// 打印版本信息
	fmt.Printf("GCS-Distill Worker %s\n", version)
	fmt.Printf("节点名称: %s\n", *nodeName)
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

	logger.Infof("GCS-Distill Worker [%s] 启动中...", *nodeName)

	// TODO: 检测系统资源 (GPU, CPU, Memory)
	// TODO: 连接到控制面
	// TODO: 启动 gRPC 服务器
	// TODO: 启动资源上报协程
	// TODO: 启动容器管理器

	logger.Infof("Worker [%s] 启动成功", *nodeName)

	// 等待信号
	waitForSignal()

	logger.Infof("Worker [%s] 关闭中...", *nodeName)
	// TODO: 优雅关闭
}

// waitForSignal 等待系统信号
func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
