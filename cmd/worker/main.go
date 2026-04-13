package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/docker"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	pb "github.com/ReyRen/gcs-distill/proto"
	"github.com/ReyRen/gcs-distill/repository/redis"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	configPath = flag.String("config", "config.yaml", "配置文件路径")
	nodeName   = flag.String("name", "", "Worker 节点名称")
	nodeAddr   = flag.String("addr", "", "Worker 节点地址 (host:port)")
	version    = "v0.1.0"
)

func main() {
	flag.Parse()

	// 打印版本信息
	fmt.Printf("GCS-Distill Worker %s\n", version)

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

	logger.Info("GCS-Distill Worker 启动中...",
		zap.String("version", version),
		zap.String("node_name", *nodeName),
		zap.String("node_addr", *nodeAddr),
	)

	// 检查必需参数
	if *nodeName == "" {
		logger.Fatal("节点名称不能为空，请使用 -name 参数指定")
	}
	if *nodeAddr == "" {
		logger.Fatal("节点地址不能为空，请使用 -addr 参数指定")
	}

	// 初始化 Docker 容器管理器
	containerMgr, err := docker.NewContainerManager()
	if err != nil {
		logger.Fatal("初始化容器管理器失败", zap.Error(err))
	}
	defer containerMgr.Close()
	logger.Info("容器管理器初始化成功")

	// 初始化 Redis（用于心跳上报）
	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		logger.Fatal("初始化Redis失败", zap.Error(err))
	}
	defer redisClient.Close()
	logger.Info("Redis连接成功")

	nodeCache := redis.NewNodeCache(redisClient)

	// 创建 gRPC 服务器
	grpcServer := grpc.NewServer()
	workerService := NewWorkerService(containerMgr)
	pb.RegisterWorkerServiceServer(grpcServer, workerService)

	// 启动 gRPC 服务器
	lis, err := net.Listen("tcp", *nodeAddr)
	if err != nil {
		logger.Fatal("监听端口失败",
			zap.String("addr", *nodeAddr),
			zap.Error(err),
		)
	}

	go func() {
		logger.Info("gRPC 服务器启动", zap.String("address", *nodeAddr))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("gRPC 服务器启动失败", zap.Error(err))
		}
	}()

	// 启动心跳协程
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startHeartbeat(ctx, nodeCache, *nodeName, *nodeAddr)

	// 启动容器清理协程
	go startContainerCleanup(ctx, containerMgr)

	logger.Info("Worker 节点启动成功")

	// 等待信号
	waitForSignal()

	logger.Info("GCS-Distill Worker 关闭中...")

	// 优雅关闭
	grpcServer.GracefulStop()

	logger.Info("Worker 节点已关闭")
}

// waitForSignal 等待系统信号
func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}

// startHeartbeat 启动心跳上报
func startHeartbeat(ctx context.Context, nodeCache redis.NodeCache, nodeName, nodeAddr string) {
	ticker := time.NewTicker(30 * time.Second) // 每30秒上报一次
	defer ticker.Stop()

	// 立即上报一次
	reportHeartbeat(ctx, nodeCache, nodeName, nodeAddr)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reportHeartbeat(ctx, nodeCache, nodeName, nodeAddr)
		}
	}
}

// reportHeartbeat 上报心跳
func reportHeartbeat(ctx context.Context, nodeCache redis.NodeCache, nodeName, nodeAddr string) {
	// TODO: 检测实际的资源使用情况
	// 这里使用示例数据

	node := &types.WorkerNode{
		NodeName:      nodeName,
		NodeAddr:      nodeAddr,
		TotalGPU:      4,      // TODO: 从系统检测
		AvailableGPU:  2,      // TODO: 动态计算
		TotalMemoryGB: 128,    // TODO: 从系统检测
		TotalCPU:      32,     // TODO: 从系统检测
	}

	if err := nodeCache.SetNode(ctx, node); err != nil {
		logger.Error("上报心跳失败",
			zap.String("node_name", nodeName),
			zap.Error(err),
		)
	} else {
		logger.Debug("心跳上报成功", zap.String("node_name", nodeName))
	}
}

// startContainerCleanup 启动容器清理
func startContainerCleanup(ctx context.Context, containerMgr *docker.ContainerManager) {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := containerMgr.CleanupExited(ctx); err != nil {
				logger.Error("清理容器失败", zap.Error(err))
			}
		}
	}
}
