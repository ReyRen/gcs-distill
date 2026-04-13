package docker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"go.uber.org/zap"
)

// ContainerManager 容器管理器
type ContainerManager struct {
	client     *Client
	containers sync.Map // containerID -> *ManagedContainer
	mu         sync.RWMutex
}

// NewContainerManager 创建容器管理器
func NewContainerManager() (*ContainerManager, error) {
	client, err := NewClient()
	if err != nil {
		return nil, err
	}

	return &ContainerManager{
		client: client,
	}, nil
}

// Close 关闭管理器
func (m *ContainerManager) Close() error {
	return m.client.Close()
}

// ManagedContainer 托管容器
type ManagedContainer struct {
	ID        string
	Config    *ContainerConfig
	Status    *ContainerStatus
	CreatedAt time.Time
	StartedAt time.Time
	mu        sync.RWMutex
}

// RunContainer 运行容器（创建+启动）
func (m *ContainerManager) RunContainer(ctx context.Context, cfg *ContainerConfig) (string, error) {
	// 创建容器
	containerID, err := m.client.CreateContainer(ctx, cfg)
	if err != nil {
		return "", err
	}

	// 记录托管容器
	managed := &ManagedContainer{
		ID:        containerID,
		Config:    cfg,
		CreatedAt: time.Now(),
	}
	m.containers.Store(containerID, managed)

	// 启动容器
	if err := m.client.StartContainer(ctx, containerID); err != nil {
		// 启动失败，清理容器
		_ = m.client.RemoveContainer(ctx, containerID)
		m.containers.Delete(containerID)
		return "", err
	}

	managed.StartedAt = time.Now()

	logger.Info("容器运行成功",
		zap.String("container_id", containerID),
		zap.String("image", cfg.Image),
	)

	return containerID, nil
}

// StopContainer 停止容器
func (m *ContainerManager) StopContainer(ctx context.Context, containerID string, timeout int) error {
	if err := m.client.StopContainer(ctx, containerID, timeout); err != nil {
		return err
	}

	// 更新状态
	if val, ok := m.containers.Load(containerID); ok {
		managed := val.(*ManagedContainer)
		managed.mu.Lock()
		status, _ := m.client.GetContainerStatus(ctx, containerID)
		managed.Status = status
		managed.mu.Unlock()
	}

	return nil
}

// RemoveContainer 删除容器
func (m *ContainerManager) RemoveContainer(ctx context.Context, containerID string) error {
	if err := m.client.RemoveContainer(ctx, containerID); err != nil {
		return err
	}

	m.containers.Delete(containerID)
	return nil
}

// GetContainerStatus 获取容器状态
func (m *ContainerManager) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	status, err := m.client.GetContainerStatus(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	if val, ok := m.containers.Load(containerID); ok {
		managed := val.(*ManagedContainer)
		managed.mu.Lock()
		managed.Status = status
		managed.mu.Unlock()
	}

	return status, nil
}

// GetContainerLogs 获取容器日志
func (m *ContainerManager) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	return m.client.GetContainerLogs(ctx, containerID, tail)
}

// ListManagedContainers 列出所有托管容器
func (m *ContainerManager) ListManagedContainers() []*ManagedContainer {
	var containers []*ManagedContainer

	m.containers.Range(func(key, value interface{}) bool {
		managed := value.(*ManagedContainer)
		containers = append(containers, managed)
		return true
	})

	return containers
}

// CleanupExited 清理已退出的容器
func (m *ContainerManager) CleanupExited(ctx context.Context) error {
	logger.Info("开始清理已退出的容器")

	var toRemove []string

	m.containers.Range(func(key, value interface{}) bool {
		containerID := key.(string)
		managed := value.(*ManagedContainer)

		// 获取最新状态
		status, err := m.client.GetContainerStatus(ctx, containerID)
		if err != nil {
			logger.Warn("获取容器状态失败",
				zap.String("container_id", containerID),
				zap.Error(err),
			)
			return true
		}

		managed.mu.Lock()
		managed.Status = status
		managed.mu.Unlock()

		// 如果容器已退出，标记删除
		if status.Status == "exited" {
			toRemove = append(toRemove, containerID)
		}

		return true
	})

	// 删除已退出的容器
	for _, containerID := range toRemove {
		if err := m.RemoveContainer(ctx, containerID); err != nil {
			logger.Warn("删除容器失败",
				zap.String("container_id", containerID),
				zap.Error(err),
			)
		} else {
			logger.Info("已清理退出容器", zap.String("container_id", containerID))
		}
	}

	logger.Info("容器清理完成", zap.Int("cleaned", len(toRemove)))
	return nil
}

// GetStats 获取容器统计信息
func (m *ContainerManager) GetStats() map[string]int {
	stats := map[string]int{
		"total":   0,
		"running": 0,
		"exited":  0,
		"other":   0,
	}

	m.containers.Range(func(key, value interface{}) bool {
		managed := value.(*ManagedContainer)
		stats["total"]++

		managed.mu.RLock()
		status := managed.Status
		managed.mu.RUnlock()

		if status == nil {
			stats["other"]++
		} else if status.Running {
			stats["running"]++
		} else if status.Status == "exited" {
			stats["exited"]++
		} else {
			stats["other"]++
		}

		return true
	})

	return stats
}

// WaitForContainer 等待容器完成
func (m *ContainerManager) WaitForContainer(ctx context.Context, containerID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待容器超时")
		case <-ticker.C:
			status, err := m.GetContainerStatus(ctx, containerID)
			if err != nil {
				return fmt.Errorf("获取容器状态失败: %w", err)
			}

			if status.Status == "exited" {
				if status.ExitCode == 0 {
					return nil
				}
				return fmt.Errorf("容器执行失败，退出码: %d, 错误: %s", status.ExitCode, status.Error)
			}

			if status.Status == "dead" {
				return fmt.Errorf("容器已死亡: %s", status.Error)
			}
		}
	}
}
