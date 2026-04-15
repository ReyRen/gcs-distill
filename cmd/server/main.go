package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/repository/postgres"
	"github.com/ReyRen/gcs-distill/repository/redis"
	"github.com/ReyRen/gcs-distill/server"
	"github.com/ReyRen/gcs-distill/service"
	"go.uber.org/zap"
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

	logger.Info("GCS-Distill Server 启动中...",
		zap.String("version", version),
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
	)

	// 初始化数据库
	db, err := postgres.NewDB(&cfg.Database)
	if err != nil {
		logger.Fatal("初始化数据库失败", zap.Error(err))
	}
	defer db.Close()
	logger.Info("数据库连接成功")

	// 初始化 Redis
	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		logger.Fatal("初始化Redis失败", zap.Error(err))
	}
	defer redisClient.Close()
	logger.Info("Redis连接成功")

	// 创建仓库层
	projectRepo := postgres.NewProjectRepository(db)
	datasetRepo := postgres.NewDatasetRepository(db)
	pipelineRepo := postgres.NewPipelineRepository(db)
	stageRepo := postgres.NewStageRepository(db)
	nodeCache := redis.NewNodeCache(redisClient)

	// 创建服务层
	projectSvc := service.NewProjectService(projectRepo)
	datasetSvc := service.NewDatasetService(datasetRepo, projectRepo, &cfg.Storage)
	schedulerSvc := service.NewSchedulerService(nodeCache)

	// 创建执行器服务
	executorSvc := service.NewExecutorService(
		pipelineRepo,
		stageRepo,
		projectRepo,
		datasetRepo,
		schedulerSvc,
		cfg.Executor.WorkspaceRoot,
		cfg.Executor.MaxConcurrent,
	)

	// 启动执行器
	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()
	executorSvc.Start(execCtx)
	defer executorSvc.Stop()

	// 创建流水线服务（注入执行器）
	pipelineSvc := service.NewPipelineService(pipelineRepo, stageRepo, projectRepo, datasetRepo, executorSvc)

	// 创建路由器
	router := server.NewRouter(projectSvc, datasetSvc, pipelineSvc, schedulerSvc)

	// 启动 HTTP 服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("HTTP服务器启动", zap.String("address", addr))

	go func() {
		if err := router.Engine().Run(addr); err != nil {
			logger.Fatal("HTTP服务器启动失败", zap.Error(err))
		}
	}()

	logger.Info("服务器启动成功")

	// 等待信号
	waitForSignal()

	logger.Info("GCS-Distill Server 关闭中...")

	// 设置关闭超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 清理资源
	_ = ctx

	logger.Info("服务器已关闭")
}

// waitForSignal 等待系统信号
func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
