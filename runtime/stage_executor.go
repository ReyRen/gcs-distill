package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	pb "github.com/ReyRen/gcs-distill/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getExtraParamString(config types.ModelConfig, key string) string {
	if config.ExtraParams == nil {
		return ""
	}
	value, ok := config.ExtraParams[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// StageExecutor 阶段执行器
type StageExecutor struct {
	configGen    *ConfigGenerator
	manifestMgr  *ManifestManager
	dataGovernor *DataGovernor
}

// NewStageExecutor 创建阶段执行器
func NewStageExecutor(workspaceRoot string) *StageExecutor {
	return &StageExecutor{
		configGen:    NewConfigGenerator(workspaceRoot),
		manifestMgr:  NewManifestManager(workspaceRoot),
		dataGovernor: NewDataGovernor(),
	}
}

// ExecuteStage 执行单个阶段
func (e *StageExecutor) ExecuteStage(
	ctx context.Context,
	stage *types.StageRun,
	pipeline *types.PipelineRun,
	project *types.Project,
	workerAddr string,
) error {
	logger.Info("开始执行阶段",
		zap.String("stage_type", string(stage.StageType)),
		zap.String("stage_id", stage.ID),
		zap.String("worker", workerAddr),
	)

	// 根据阶段类型执行不同逻辑
	switch stage.StageType {
	case types.StageTeacherConfig:
		return e.executeTeacherConfig(ctx, stage, project, pipeline)
	case types.StageDatasetBuild:
		return e.executeDatasetBuild(ctx, stage, project, pipeline)
	case types.StageTeacherInfer:
		return e.executeTeacherInfer(ctx, stage, project, pipeline, workerAddr)
	case types.StageDataGovern:
		return e.executeDataGovern(ctx, stage, project, pipeline)
	case types.StageStudentTrain:
		return e.executeStudentTrain(ctx, stage, project, pipeline, workerAddr)
	case types.StageEvaluate:
		return e.executeEvaluate(ctx, stage, project, pipeline, workerAddr)
	default:
		return fmt.Errorf("未知的阶段类型: %s", stage.StageType)
	}
}

// executeTeacherConfig 执行阶段1: 教师模型配置验证
func (e *StageExecutor) executeTeacherConfig(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
) error {
	logger.Info("验证教师模型配置")

	// 验证教师模型配置
	if project.TeacherModelConfig.ModelName == "" {
		return fmt.Errorf("教师模型配置为空")
	}

	config := project.TeacherModelConfig

	// 基本验证
	if config.ModelName == "" {
		return fmt.Errorf("教师模型名称不能为空")
	}

	if config.ProviderType == "" {
		return fmt.Errorf("教师模型提供商类型不能为空")
	}

	// API 类型验证
	if config.ProviderType == types.ProviderAPI {
		if strings.TrimSpace(config.Endpoint) == "" {
			return fmt.Errorf("API 类型教师模型需要提供 endpoint")
		}
		if strings.TrimSpace(config.APISecretRef) == "" {
			return fmt.Errorf("API 类型教师模型需要提供 api_secret_ref")
		}
	}

	// 本地类型验证
	if config.ProviderType == types.ProviderLocal {
		if getExtraParamString(config, "model_path") == "" {
			return fmt.Errorf("本地类型教师模型需要提供 model_path")
		}
	}

	logger.Info("教师模型配置验证通过",
		zap.String("model", config.ModelName),
		zap.String("provider", string(config.ProviderType)),
	)

	// 保存配置到清单
	stage.OutputManifest = map[string]string{
		"teacher_model": config.ModelName,
		"provider_type": string(config.ProviderType),
		"validated_at":  time.Now().Format(time.RFC3339),
	}

	return nil
}

// executeDatasetBuild 执行阶段2: 数据集构建
func (e *StageExecutor) executeDatasetBuild(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
) error {
	logger.Info("构建数据集清单")

	// 这里假设数据集已经上传到共享存储
	// 实际实现中，这里需要读取用户上传的数据集文件
	// 并转换为 EasyDistill 期望的格式

	projectID := project.ID
	runID := pipeline.ID

	// 创建工作空间目录
	workspace := e.configGen.GetRunWorkspace(projectID, runID)
	dirs := []string{
		filepath.Join(workspace, "data", "seed"),
		filepath.Join(workspace, "data", "generated"),
		filepath.Join(workspace, "data", "filtered"),
		filepath.Join(workspace, "configs"),
		filepath.Join(workspace, "logs"),
		filepath.Join(workspace, "models", "checkpoints"),
		filepath.Join(workspace, "eval"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}
	}

	logger.Info("工作空间目录创建完成", zap.String("workspace", workspace))

	// 示例：创建示例种子数据（实际应该从数据集加载）
	// TODO: 从实际数据集ID加载数据
	instructions := []Instruction{
		{
			Instruction: "解释什么是机器学习",
			Input:       "",
		},
		{
			Instruction: "用Python写一个快速排序",
			Input:       "",
		},
	}

	if err := e.manifestMgr.CreateSeedManifest(projectID, runID, instructions); err != nil {
		return fmt.Errorf("创建种子数据清单失败: %w", err)
	}

	logger.Info("种子数据清单创建完成", zap.Int("count", len(instructions)))

	// 保存清单信息
	stage.OutputManifest = map[string]string{
		"seed_count": fmt.Sprintf("%d", len(instructions)),
		"workspace":  workspace,
		"created_at": time.Now().Format(time.RFC3339),
	}

	return nil
}

// executeTeacherInfer 执行阶段3: 教师模型推理
func (e *StageExecutor) executeTeacherInfer(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
	workerAddr string,
) error {
	logger.Info("执行教师模型推理")

	projectID := project.ID
	runID := pipeline.ID

	// 生成配置文件
	configData, err := e.configGen.GenerateTeacherInferConfig(project, runID)
	if err != nil {
		return fmt.Errorf("生成教师推理配置失败: %w", err)
	}

	// 保存配置文件
	configPath := e.configGen.GetConfigPath(projectID, runID, "teacher_infer")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}
	stage.ConfigPath = configPath

	logger.Info("配置文件已生成", zap.String("config", configPath))

	// 调用 Worker 执行容器
	containerID, err := e.runDockerContainer(ctx, workerAddr, &ContainerRequest{
		Image:       "gcs-distill/easydistill:latest",
		Command:     []string{"--config", "/workspace/configs/teacher_infer.json"},
		WorkDir:     "/workspace",
		VolumeMounts: map[string]string{
			e.configGen.GetRunWorkspace(projectID, runID): "/workspace",
		},
		GPUs:    pipeline.ResourceRequest.GPUCount,
		Memory:  pipeline.ResourceRequest.MemoryGB,
		CPUs:    pipeline.ResourceRequest.CPUCores,
	})

	if err != nil {
		return fmt.Errorf("启动容器失败: %w", err)
	}

	logger.Info("容器已启动", zap.String("container_id", containerID))

	// 等待容器完成
	if err := e.waitForContainer(ctx, workerAddr, containerID); err != nil {
		return fmt.Errorf("容器执行失败: %w", err)
	}

	logger.Info("教师模型推理完成")

	// 统计生成的数据
	stats, _ := e.manifestMgr.GetManifestStats(projectID, runID)

	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "teacher_infer")
	stage.OutputManifest = map[string]string{
		"container_id":  containerID,
		"labeled_count": fmt.Sprintf("%d", stats["labeled"]),
		"config_path":   configPath,
	}

	return nil
}

// executeDataGovern 执行阶段4: 数据治理
func (e *StageExecutor) executeDataGovern(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
) error {
	logger.Info("执行数据治理")

	projectID := project.ID
	runID := pipeline.ID

	// 加载标注数据
	labeled, err := e.manifestMgr.LoadLabeledData(projectID, runID)
	if err != nil {
		return fmt.Errorf("加载标注数据失败: %w", err)
	}

	logger.Info("标注数据加载完成", zap.Int("count", len(labeled)))

	// 数据治理
	train, test, stats := e.dataGovernor.FilterData(labeled)

	logger.Info(e.dataGovernor.GetFilterStats(stats))

	// 保存过滤后的数据
	if err := e.manifestMgr.SaveFilteredData(projectID, runID, train, test); err != nil {
		return fmt.Errorf("保存过滤数据失败: %w", err)
	}

	logger.Info("数据治理完成",
		zap.Int("train", len(train)),
		zap.Int("test", len(test)),
	)

	// 保存统计信息
	filterRate := 0.0
	if stats["total"] > 0 {
		filterRate = float64(stats["filtered"]) / float64(stats["total"])
	}
	stage.OutputManifest = map[string]string{
		"train_count": fmt.Sprintf("%d", len(train)),
		"test_count":  fmt.Sprintf("%d", len(test)),
		"filter_rate": fmt.Sprintf("%.4f", filterRate),
	}
	stage.Metrics = map[string]interface{}{
		"stats":       stats,
		"train_count": len(train),
		"test_count":  len(test),
		"filter_rate": filterRate,
	}

	return nil
}

// executeStudentTrain 执行阶段5: 学生模型训练
func (e *StageExecutor) executeStudentTrain(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
	workerAddr string,
) error {
	logger.Info("执行学生模型训练")

	projectID := project.ID
	runID := pipeline.ID

	// 生成训练配置
	configData, err := e.configGen.GenerateStudentTrainConfig(project, pipeline, runID)
	if err != nil {
		return fmt.Errorf("生成训练配置失败: %w", err)
	}

	// 保存配置文件
	configPath := e.configGen.GetConfigPath(projectID, runID, "student_train")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}
	stage.ConfigPath = configPath

	logger.Info("训练配置已生成", zap.String("config", configPath))

	// 调用 Worker 执行训练容器
	containerID, err := e.runDockerContainer(ctx, workerAddr, &ContainerRequest{
		Image:       "gcs-distill/easydistill:latest",
		Command:     []string{"--config", "/workspace/configs/student_train.json"},
		WorkDir:     "/workspace",
		VolumeMounts: map[string]string{
			e.configGen.GetRunWorkspace(projectID, runID): "/workspace",
		},
		GPUs:    pipeline.ResourceRequest.GPUCount,
		Memory:  pipeline.ResourceRequest.MemoryGB,
		CPUs:    pipeline.ResourceRequest.CPUCores,
	})

	if err != nil {
		return fmt.Errorf("启动训练容器失败: %w", err)
	}

	logger.Info("训练容器已启动", zap.String("container_id", containerID))

	// 等待容器完成（训练可能需要很长时间）
	if err := e.waitForContainer(ctx, workerAddr, containerID); err != nil {
		return fmt.Errorf("训练失败: %w", err)
	}

	logger.Info("学生模型训练完成")

	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "student_train")
	stage.OutputManifest = map[string]string{
		"container_id":    containerID,
		"checkpoint_path": "/workspace/models/checkpoints/",
		"config_path":     configPath,
	}

	return nil
}

// executeEvaluate 执行阶段6: 模型评估
func (e *StageExecutor) executeEvaluate(
	ctx context.Context,
	stage *types.StageRun,
	project *types.Project,
	pipeline *types.PipelineRun,
	workerAddr string,
) error {
	logger.Info("执行模型评估")

	projectID := project.ID
	runID := pipeline.ID

	// 生成评估配置
	configData, err := e.configGen.GenerateEvaluateConfig(project, runID)
	if err != nil {
		return fmt.Errorf("生成评估配置失败: %w", err)
	}

	// 保存配置文件
	configPath := e.configGen.GetConfigPath(projectID, runID, "evaluate")
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}
	stage.ConfigPath = configPath

	logger.Info("评估配置已生成", zap.String("config", configPath))

	// 调用 Worker 执行评估容器
	containerID, err := e.runDockerContainer(ctx, workerAddr, &ContainerRequest{
		Image:       "gcs-distill/easydistill:latest",
		Command:     []string{"--config", "/workspace/configs/evaluate.json"},
		WorkDir:     "/workspace",
		VolumeMounts: map[string]string{
			e.configGen.GetRunWorkspace(projectID, runID): "/workspace",
		},
		GPUs:    1, // 评估只需要1个GPU
		Memory:  pipeline.ResourceRequest.MemoryGB,
		CPUs:    pipeline.ResourceRequest.CPUCores,
	})

	if err != nil {
		return fmt.Errorf("启动评估容器失败: %w", err)
	}

	logger.Info("评估容器已启动", zap.String("container_id", containerID))

	// 等待容器完成
	if err := e.waitForContainer(ctx, workerAddr, containerID); err != nil {
		return fmt.Errorf("评估失败: %w", err)
	}

	logger.Info("模型评估完成")

	// TODO: 解析评估结果并保存到 stage.Metrics

	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "evaluate")
	stage.OutputManifest = map[string]string{
		"container_id": containerID,
		"result_path":  "/workspace/eval/results.json",
		"config_path":  configPath,
	}

	return nil
}

// ContainerRequest 容器请求
type ContainerRequest struct {
	Image        string
	Command      []string
	WorkDir      string
	VolumeMounts map[string]string
	GPUs         int
	GPUDeviceIDs string // GPU 设备 ID，如 "0,1,2"
	Memory       int
	CPUs         int
}

// runDockerContainer 在 Worker 节点上运行 Docker 容器
func (e *StageExecutor) runDockerContainer(
	ctx context.Context,
	workerAddr string,
	req *ContainerRequest,
) (string, error) {
	// 连接到 Worker 的 gRPC 服务
	conn, err := grpc.Dial(workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("连接Worker失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewWorkerServiceClient(conn)

	// 构建卷挂载
	var volumes []*pb.VolumeMount
	for host, container := range req.VolumeMounts {
		volumes = append(volumes, &pb.VolumeMount{
			HostPath:      host,
			ContainerPath: container,
		})
	}

	// 调用 RunContainer
	resp, err := client.RunContainer(ctx, &pb.RunContainerRequest{
		Image:        req.Image,
		Command:      req.Command,
		WorkDir:      req.WorkDir,
		VolumeMounts: volumes,
		GpuCount:     int32(req.GPUs),
		GpuDeviceIds: req.GPUDeviceIDs,
		MemoryGb:     int32(req.Memory),
		CpuCores:     int32(req.CPUs),
	})

	if err != nil {
		return "", fmt.Errorf("启动容器失败: %w", err)
	}

	return resp.ContainerId, nil
}

// waitForContainer 等待容器执行完成
func (e *StageExecutor) waitForContainer(
	ctx context.Context,
	workerAddr string,
	containerID string,
) error {
	// 连接到 Worker
	conn, err := grpc.Dial(workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("连接Worker失败: %w", err)
	}
	defer conn.Close()

	client := pb.NewWorkerServiceClient(conn)

	// 轮询容器状态
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待容器超时")
		case <-ticker.C:
			// 获取容器状态
			statusResp, err := client.GetContainerStatus(ctx, &pb.GetContainerStatusRequest{
				ContainerId: containerID,
			})
			if err != nil {
				logger.Warn("获取容器状态失败", zap.Error(err))
				continue
			}

			logger.Info("容器状态",
				zap.String("container_id", containerID),
				zap.String("status", statusResp.Status),
			)

			// 检查状态
			if statusResp.Status == "exited" {
				if statusResp.ExitCode == 0 {
					logger.Info("容器执行成功", zap.String("container_id", containerID))
					return nil
				}
				return fmt.Errorf("容器执行失败，退出码: %d", statusResp.ExitCode)
			}

			if statusResp.Status == "error" || statusResp.Status == "failed" {
				return fmt.Errorf("容器执行失败: %s", statusResp.Error)
			}
		}
	}
}

// ReadLogFile 从工作空间读取日志文件
func (e *StageExecutor) ReadLogFile(projectID, runID, stageName string) (string, error) {
	logPath := e.configGen.GetLogPath(projectID, runID, stageName)

	// 检查日志文件是否存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("日志文件不存在: %s", logPath)
	}

	// 读取日志文件
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	return string(content), nil
}

// TailLogFile 读取日志文件的最后 N 行
func (e *StageExecutor) TailLogFile(projectID, runID, stageName string, lines int) (string, error) {
	logPath := e.configGen.GetLogPath(projectID, runID, stageName)

	// 检查日志文件是否存在
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return "", fmt.Errorf("日志文件不存在: %s", logPath)
	}

	// 读取整个文件
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %w", err)
	}

	// 按行分割
	allLines := strings.Split(string(content), "\n")

	// 获取最后 N 行
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	return strings.Join(allLines[start:], "\n"), nil
}
