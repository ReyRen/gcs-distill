package main

import (
	"context"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/docker"
	"github.com/ReyRen/gcs-distill/internal/logger"
	pb "github.com/ReyRen/gcs-distill/proto"
	"go.uber.org/zap"
)

// WorkerService Worker 服务实现
type WorkerService struct {
	pb.UnimplementedWorkerServiceServer
	containerMgr *docker.ContainerManager
}

// NewWorkerService 创建 Worker 服务
func NewWorkerService(containerMgr *docker.ContainerManager) *WorkerService {
	return &WorkerService{
		containerMgr: containerMgr,
	}
}

// RunContainer 运行容器
func (s *WorkerService) RunContainer(ctx context.Context, req *pb.RunContainerRequest) (*pb.RunContainerResponse, error) {
	logger.Info("接收到容器运行请求",
		zap.String("image", req.Image),
		zap.Strings("command", req.Command),
	)

	// 转换配置
	cfg := &docker.ContainerConfig{
		Image:    req.Image,
		Command:  req.Command,
		WorkDir:  req.WorkDir,
		Env:      req.Env,
		GPUCount: int(req.GpuCount),
		MemoryGB: int(req.MemoryGb),
		CPUCores: int(req.CpuCores),
		Labels: map[string]string{
			"managed-by": "gcs-distill",
		},
	}

	// 转换卷挂载
	for _, mount := range req.VolumeMounts {
		cfg.Mounts = append(cfg.Mounts, docker.Mount{
			Source: mount.HostPath,
			Target: mount.ContainerPath,
			Type:   "bind",
		})
	}

	// 运行容器
	containerID, err := s.containerMgr.RunContainer(ctx, cfg)
	if err != nil {
		logger.Error("运行容器失败", zap.Error(err))
		return nil, fmt.Errorf("运行容器失败: %w", err)
	}

	logger.Info("容器运行成功",
		zap.String("container_id", containerID),
		zap.String("image", req.Image),
	)

	return &pb.RunContainerResponse{
		ContainerId: containerID,
	}, nil
}

// GetContainerStatus 获取容器状态
func (s *WorkerService) GetContainerStatus(ctx context.Context, req *pb.GetContainerStatusRequest) (*pb.GetContainerStatusResponse, error) {
	status, err := s.containerMgr.GetContainerStatus(ctx, req.ContainerId)
	if err != nil {
		logger.Error("获取容器状态失败",
			zap.String("container_id", req.ContainerId),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取容器状态失败: %w", err)
	}

	return &pb.GetContainerStatusResponse{
		Status:     status.Status,
		Running:    status.Running,
		ExitCode:   int32(status.ExitCode),
		Error:      status.Error,
		StartedAt:  status.StartedAt,
		FinishedAt: status.FinishedAt,
	}, nil
}

// GetContainerLogs 获取容器日志
func (s *WorkerService) GetContainerLogs(ctx context.Context, req *pb.GetContainerLogsRequest) (*pb.GetContainerLogsResponse, error) {
	tail := int(req.TailLines)
	if tail <= 0 {
		tail = 100 // 默认100行
	}

	logs, err := s.containerMgr.GetContainerLogs(ctx, req.ContainerId, tail)
	if err != nil {
		logger.Error("获取容器日志失败",
			zap.String("container_id", req.ContainerId),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取容器日志失败: %w", err)
	}

	return &pb.GetContainerLogsResponse{
		Logs: logs,
	}, nil
}

// StopContainer 停止容器
func (s *WorkerService) StopContainer(ctx context.Context, req *pb.StopContainerRequest) (*pb.StopContainerResponse, error) {
	logger.Info("停止容器", zap.String("container_id", req.ContainerId))

	timeout := int(req.TimeoutSeconds)
	if timeout <= 0 {
		timeout = 10 // 默认10秒
	}

	if err := s.containerMgr.StopContainer(ctx, req.ContainerId, timeout); err != nil {
		logger.Error("停止容器失败",
			zap.String("container_id", req.ContainerId),
			zap.Error(err),
		)
		return nil, fmt.Errorf("停止容器失败: %w", err)
	}

	return &pb.StopContainerResponse{
		Success: true,
	}, nil
}

// ReportResources 上报资源（未来扩展）
func (s *WorkerService) ReportResources(ctx context.Context, req *pb.ReportResourcesRequest) (*pb.ReportResourcesResponse, error) {
	// 获取容器统计
	stats := s.containerMgr.GetStats()

	logger.Info("资源上报",
		zap.Int("total_containers", stats["total"]),
		zap.Int("running_containers", stats["running"]),
	)

	return &pb.ReportResourcesResponse{
		Success: true,
	}, nil
}
