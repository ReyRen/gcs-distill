package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/repository/redis"
	"go.uber.org/zap"
)

// SchedulerService 调度服务接口
type SchedulerService interface {
	// RegisterNode 注册 Worker 节点
	RegisterNode(ctx context.Context, node *types.WorkerNode) error
	// UnregisterNode 注销 Worker 节点
	UnregisterNode(ctx context.Context, nodeName string) error
	// UpdateNodeHeartbeat 更新节点心跳
	UpdateNodeHeartbeat(ctx context.Context, nodeName string) error
	// GetNode 获取节点信息
	GetNode(ctx context.Context, nodeName string) (*types.WorkerNode, error)
	// ListNodes 列出所有在线节点
	ListNodes(ctx context.Context) ([]*types.WorkerNode, error)
	// FindAvailableNode 查找可用节点
	FindAvailableNode(ctx context.Context, resourceReq types.ResourceRequest) (*types.WorkerNode, error)
	// AllocateResources 分配资源
	AllocateResources(ctx context.Context, nodeName string, resourceReq types.ResourceRequest) error
	// ReleaseResources 释放资源
	ReleaseResources(ctx context.Context, nodeName string, resourceReq types.ResourceRequest) error
	// CleanExpiredNodes 清理过期节点
	CleanExpiredNodes(ctx context.Context) error
}

// schedulerService 调度服务实现
type schedulerService struct {
	nodeCache redis.NodeCache
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(nodeCache redis.NodeCache) SchedulerService {
	return &schedulerService{
		nodeCache: nodeCache,
	}
}

// RegisterNode 注册 Worker 节点
func (s *schedulerService) RegisterNode(ctx context.Context, node *types.WorkerNode) error {
	// 设置节点状态
	node.Status = "online"
	node.LastHeartbeat = time.Now()
	node.UpdatedAt = time.Now()

	// 保存节点信息
	if err := s.nodeCache.SetNode(ctx, node); err != nil {
		logger.Error("注册节点失败",
			zap.String("node_name", node.NodeName),
			zap.Error(err),
		)
		return fmt.Errorf("注册节点失败: %w", err)
	}

	logger.Info("节点注册成功",
		zap.String("node_name", node.NodeName),
		zap.String("node_addr", node.NodeAddr),
		zap.Int("total_gpu", node.TotalGPU),
	)

	return nil
}

// UnregisterNode 注销 Worker 节点
func (s *schedulerService) UnregisterNode(ctx context.Context, nodeName string) error {
	if err := s.nodeCache.DeleteNode(ctx, nodeName); err != nil {
		logger.Error("注销节点失败",
			zap.String("node_name", nodeName),
			zap.Error(err),
		)
		return fmt.Errorf("注销节点失败: %w", err)
	}

	logger.Info("节点注销成功", zap.String("node_name", nodeName))

	return nil
}

// UpdateNodeHeartbeat 更新节点心跳
func (s *schedulerService) UpdateNodeHeartbeat(ctx context.Context, nodeName string) error {
	// 获取节点信息
	node, err := s.nodeCache.GetNode(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("节点不存在: %s", nodeName)
	}

	// 更新心跳时间
	node.LastHeartbeat = time.Now()
	node.UpdatedAt = time.Now()

	// 保存节点信息
	if err := s.nodeCache.SetNode(ctx, node); err != nil {
		return fmt.Errorf("更新节点心跳失败: %w", err)
	}

	return nil
}

// GetNode 获取节点信息
func (s *schedulerService) GetNode(ctx context.Context, nodeName string) (*types.WorkerNode, error) {
	node, err := s.nodeCache.GetNode(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("获取节点信息失败: %w", err)
	}

	return node, nil
}

// ListNodes 列出所有在线节点
func (s *schedulerService) ListNodes(ctx context.Context) ([]*types.WorkerNode, error) {
	nodes, err := s.nodeCache.ListNodes(ctx)
	if err != nil {
		logger.Error("获取节点列表失败", zap.Error(err))
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	return nodes, nil
}

// FindAvailableNode 查找可用节点
func (s *schedulerService) FindAvailableNode(ctx context.Context, resourceReq types.ResourceRequest) (*types.WorkerNode, error) {
	// 获取所有在线节点
	nodes, err := s.nodeCache.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有可用的 Worker 节点")
	}

	// 查找满足资源要求的节点
	var bestNode *types.WorkerNode
	var maxScore float64

	for _, node := range nodes {
		// 检查节点状态
		if node.Status != "online" {
			continue
		}

		// 检查 GPU 资源
		if resourceReq.GPUCount > 0 && node.AvailableGPU < resourceReq.GPUCount {
			continue
		}

		// 检查内存资源
		if resourceReq.MemoryGB > 0 && node.TotalMemoryGB < resourceReq.MemoryGB {
			continue
		}

		// 检查 CPU 资源
		if resourceReq.CPUCores > 0 && node.TotalCPU < resourceReq.CPUCores {
			continue
		}

		// 计算节点得分（资源利用率越低，得分越高）
		gpuScore := float64(node.AvailableGPU) / float64(node.TotalGPU)
		score := gpuScore

		if score > maxScore {
			maxScore = score
			bestNode = node
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("没有满足资源要求的节点 (GPU: %d)", resourceReq.GPUCount)
	}

	logger.Info("找到可用节点",
		zap.String("node_name", bestNode.NodeName),
		zap.Int("available_gpu", bestNode.AvailableGPU),
		zap.Float64("score", maxScore),
	)

	return bestNode, nil
}

// AllocateResources 分配资源
func (s *schedulerService) AllocateResources(ctx context.Context, nodeName string, resourceReq types.ResourceRequest) error {
	// 获取节点信息
	node, err := s.nodeCache.GetNode(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("节点不存在: %s", nodeName)
	}

	// 检查资源是否充足
	if resourceReq.GPUCount > 0 && node.AvailableGPU < resourceReq.GPUCount {
		return fmt.Errorf("节点 GPU 资源不足: 需要 %d, 可用 %d", resourceReq.GPUCount, node.AvailableGPU)
	}

	// 分配资源
	node.AvailableGPU -= resourceReq.GPUCount
	node.UpdatedAt = time.Now()

	// 如果资源已满，更新节点状态
	if node.AvailableGPU == 0 {
		node.Status = "busy"
	}

	// 保存节点信息
	if err := s.nodeCache.SetNode(ctx, node); err != nil {
		return fmt.Errorf("更新节点资源失败: %w", err)
	}

	logger.Info("资源分配成功",
		zap.String("node_name", nodeName),
		zap.Int("allocated_gpu", resourceReq.GPUCount),
		zap.Int("remaining_gpu", node.AvailableGPU),
	)

	return nil
}

// ReleaseResources 释放资源
func (s *schedulerService) ReleaseResources(ctx context.Context, nodeName string, resourceReq types.ResourceRequest) error {
	// 获取节点信息
	node, err := s.nodeCache.GetNode(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("节点不存在: %s", nodeName)
	}

	// 释放资源
	node.AvailableGPU += resourceReq.GPUCount
	if node.AvailableGPU > node.TotalGPU {
		node.AvailableGPU = node.TotalGPU
	}

	// 更新节点状态
	if node.AvailableGPU > 0 {
		node.Status = "online"
	}

	node.UpdatedAt = time.Now()

	// 保存节点信息
	if err := s.nodeCache.SetNode(ctx, node); err != nil {
		return fmt.Errorf("更新节点资源失败: %w", err)
	}

	logger.Info("资源释放成功",
		zap.String("node_name", nodeName),
		zap.Int("released_gpu", resourceReq.GPUCount),
		zap.Int("available_gpu", node.AvailableGPU),
	)

	return nil
}

// CleanExpiredNodes 清理过期节点
func (s *schedulerService) CleanExpiredNodes(ctx context.Context) error {
	// 节点心跳超时时间 (3 分钟)
	timeout := 3 * time.Minute

	// 检查过期节点
	expiredNodes, err := s.nodeCache.CheckExpiredNodes(ctx, timeout)
	if err != nil {
		return fmt.Errorf("检查过期节点失败: %w", err)
	}

	if len(expiredNodes) > 0 {
		logger.Warn("清理过期节点",
			zap.Int("count", len(expiredNodes)),
			zap.Strings("nodes", expiredNodes),
		)
	}

	return nil
}
