package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"github.com/ReyRen/gcs-distill/server"
	"github.com/ReyRen/gcs-distill/service"
	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "config.toml", "config file path")
	version    = "v0.1.0"
)

func main() {
	flag.Parse()

	fmt.Printf("GCS-Distill Server %s\n", version)
	fmt.Printf("Loading config: %s\n", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Initialize(&cfg.Logging); err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger failed: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("GCS-Distill Server starting",
		zap.String("version", version),
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
	)

	db, err := mysqlrepo.NewDB(&cfg.Database)
	if err != nil {
		logger.Fatal("initialize database failed", zap.Error(err))
	}
	defer db.Close()
	logger.Info("database connected")

	projectRepo := mysqlrepo.NewProjectRepository(db)
	datasetRepo := mysqlrepo.NewDatasetRepository(db)
	pipelineRepo := mysqlrepo.NewPipelineRepository(db)
	stageRepo := mysqlrepo.NewStageRepository(db)

	modelSvc := service.NewModelService(&cfg.Storage)
	projectSvc := service.NewProjectService(projectRepo, modelSvc)
	datasetSvc := service.NewDatasetService(datasetRepo, projectRepo, &cfg.Storage)
	gcsClient := gcsclient.NewClient(cfg.GCS.BaseURL, time.Duration(cfg.GCS.TimeoutSeconds)*time.Second)

	executorSvc := service.NewExecutorService(
		pipelineRepo,
		stageRepo,
		projectRepo,
		datasetRepo,
		cfg.Executor.WorkspaceRoot,
		cfg.Executor.MaxConcurrent,
		gcsClient,
		cfg.Executor.RuntimeImage,
	)

	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()
	executorSvc.Start(execCtx)
	defer executorSvc.Stop()

	pipelineSvc := service.NewPipelineService(pipelineRepo, stageRepo, projectRepo, datasetRepo, executorSvc)
	router := server.NewRouter(projectSvc, datasetSvc, pipelineSvc, modelSvc, gcsClient)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("HTTP server starting", zap.String("address", addr))

	go func() {
		if err := router.Engine().Run(addr); err != nil {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	logger.Info("server started")
	waitForSignal()
	logger.Info("GCS-Distill Server stopping")
}

func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
