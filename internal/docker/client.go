package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"go.uber.org/zap"
)

// Client Docker 客户端封装
type Client struct {
	cli *client.Client
}

// NewClient 创建 Docker 客户端
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("创建Docker客户端失败: %w", err)
	}

	return &Client{
		cli: cli,
	}, nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	return c.cli.Close()
}

// ContainerConfig 容器配置
type ContainerConfig struct {
	Image        string            // 镜像名称
	Command      []string          // 命令
	WorkDir      string            // 工作目录
	Env          []string          // 环境变量
	Mounts       []Mount           // 卷挂载
	GPUCount     int               // GPU 数量
	GPUDeviceIDs string            // GPU 设备 ID，如 "0,1,2" 或 "0"
	MemoryGB     int               // 内存（GB）
	CPUCores     int               // CPU 核心数
	Labels       map[string]string // 标签
}

// Mount 卷挂载配置
type Mount struct {
	Source string // 主机路径
	Target string // 容器路径
	Type   string // 类型（bind, volume）
}

// CreateContainer 创建容器
func (c *Client) CreateContainer(ctx context.Context, cfg *ContainerConfig) (string, error) {
	logger.Info("创建容器",
		zap.String("image", cfg.Image),
		zap.Strings("command", cfg.Command),
	)

	// 构建挂载配置
	var mounts []mount.Mount
	for _, m := range cfg.Mounts {
		mountType := mount.TypeBind
		if m.Type == "volume" {
			mountType = mount.TypeVolume
		}

		mounts = append(mounts, mount.Mount{
			Type:   mountType,
			Source: m.Source,
			Target: m.Target,
		})
	}

	// 构建容器配置
	containerCfg := &container.Config{
		Image:      cfg.Image,
		Cmd:        cfg.Command,
		WorkingDir: cfg.WorkDir,
		Env:        cfg.Env,
		Labels:     cfg.Labels,
	}

	// 构建主机配置
	hostCfg := &container.HostConfig{
		Mounts: mounts,
	}

	// GPU 配置
	if cfg.GPUCount > 0 || cfg.GPUDeviceIDs != "" {
		deviceReq := container.DeviceRequest{
			Capabilities: [][]string{{"gpu"}},
		}

		// 如果指定了具体的 GPU 设备 ID
		if cfg.GPUDeviceIDs != "" {
			deviceReq.DeviceIDs = []string{cfg.GPUDeviceIDs}
		} else if cfg.GPUCount == -1 {
			// -1 表示所有 GPU
			deviceReq.Count = -1
		} else {
			// 指定数量的 GPU
			deviceReq.Count = cfg.GPUCount
		}

		hostCfg.DeviceRequests = []container.DeviceRequest{deviceReq}
	}

	// 内存配置
	if cfg.MemoryGB > 0 {
		hostCfg.Memory = int64(cfg.MemoryGB) * 1024 * 1024 * 1024
	}

	// CPU 配置
	if cfg.CPUCores > 0 {
		hostCfg.NanoCPUs = int64(cfg.CPUCores) * 1e9
	}

	// 创建容器
	resp, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("创建容器失败: %w", err)
	}

	logger.Info("容器创建成功", zap.String("container_id", resp.ID))
	return resp.ID, nil
}

// StartContainer 启动容器
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	logger.Info("启动容器", zap.String("container_id", containerID))

	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("启动容器失败: %w", err)
	}

	logger.Info("容器启动成功", zap.String("container_id", containerID))
	return nil
}

// StopContainer 停止容器
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout int) error {
	logger.Info("停止容器",
		zap.String("container_id", containerID),
		zap.Int("timeout", timeout),
	)

	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := c.cli.ContainerStop(ctx, containerID, stopOptions); err != nil {
		return fmt.Errorf("停止容器失败: %w", err)
	}

	logger.Info("容器停止成功", zap.String("container_id", containerID))
	return nil
}

// RemoveContainer 删除容器
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	logger.Info("删除容器", zap.String("container_id", containerID))

	if err := c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("删除容器失败: %w", err)
	}

	logger.Info("容器删除成功", zap.String("container_id", containerID))
	return nil
}

// GetContainerStatus 获取容器状态
func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("检查容器状态失败: %w", err)
	}

	status := &ContainerStatus{
		ID:         inspect.ID,
		Status:     inspect.State.Status,
		Running:    inspect.State.Running,
		ExitCode:   inspect.State.ExitCode,
		Error:      inspect.State.Error,
		StartedAt:  inspect.State.StartedAt,
		FinishedAt: inspect.State.FinishedAt,
	}

	return status, nil
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	ID         string
	Status     string // created, running, exited, etc.
	Running    bool
	ExitCode   int
	Error      string
	StartedAt  string
	FinishedAt string
}

// GetContainerLogs 获取容器日志
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	}

	reader, err := c.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("获取容器日志失败: %w", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("读取容器日志失败: %w", err)
	}

	return string(logs), nil
}

// PullImage 拉取镜像
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	logger.Info("拉取镜像", zap.String("image", imageName))

	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}
	defer reader.Close()

	// 读取拉取进度（简化版本，只读取到结束）
	_, err = io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("读取镜像拉取进度失败: %w", err)
	}

	logger.Info("镜像拉取成功", zap.String("image", imageName))
	return nil
}

// ListContainers 列出容器
func (c *Client) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, fmt.Errorf("列出容器失败: %w", err)
	}

	return containers, nil
}
