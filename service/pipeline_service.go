package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/repository/postgres"
	"go.uber.org/zap"
)

// PipelineService 流水线服务接口
type PipelineService interface {
	// CreatePipeline 创建流水线运行
	CreatePipeline(ctx context.Context, pipeline *types.PipelineRun) error
	// GetPipeline 获取流水线运行
	GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error)
	// ListPipelines 列出项目的流水线运行
	ListPipelines(ctx context.Context, projectID string, page, pageSize int) ([]*types.PipelineRun, int, error)
	// StartPipeline 启动流水线
	StartPipeline(ctx context.Context, id string) error
	// CancelPipeline 取消流水线
	CancelPipeline(ctx context.Context, id string) error
	// AdvanceStage 推进到下一阶段
	AdvanceStage(ctx context.Context, pipelineID string) error
	// UpdatePipelineStatus 更新流水线状态
	UpdatePipelineStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	// ListStages 列出流水线的所有阶段
	ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error)
	// CreateStage 创建阶段运行
	CreateStage(ctx context.Context, stage *types.StageRun) error
	// UpdateStage 更新阶段运行
	UpdateStage(ctx context.Context, stage *types.StageRun) error
}

// pipelineService 流水线服务实现
type pipelineService struct {
	pipelineRepo postgres.PipelineRepository
	stageRepo    postgres.StageRepository
	projectRepo  postgres.ProjectRepository
	datasetRepo  postgres.DatasetRepository
}

// NewPipelineService 创建流水线服务
func NewPipelineService(
	pipelineRepo postgres.PipelineRepository,
	stageRepo postgres.StageRepository,
	projectRepo postgres.ProjectRepository,
	datasetRepo postgres.DatasetRepository,
) PipelineService {
	return &pipelineService{
		pipelineRepo: pipelineRepo,
		stageRepo:    stageRepo,
		projectRepo:  projectRepo,
		datasetRepo:  datasetRepo,
	}
}

// CreatePipeline 创建流水线运行
func (s *pipelineService) CreatePipeline(ctx context.Context, pipeline *types.PipelineRun) error {
	// 验证流水线信息
	if err := s.validatePipeline(ctx, pipeline); err != nil {
		return err
	}

	// 设置初始状态
	pipeline.Status = types.StatusPending
	pipeline.CurrentStage = 0

	// 创建流水线
	if err := s.pipelineRepo.Create(ctx, pipeline); err != nil {
		logger.Error("创建流水线失败",
			zap.String("project_id", pipeline.ProjectID),
			zap.Error(err),
		)
		return fmt.Errorf("创建流水线失败: %w", err)
	}

	// 创建六个阶段
	stages := []types.StageType{
		types.StageTeacherConfig,
		types.StageDatasetBuild,
		types.StageTeacherInfer,
		types.StageDataGovern,
		types.StageStudentTrain,
		types.StageEvaluate,
	}

	for i, stageType := range stages {
		stage := &types.StageRun{
			PipelineRunID: pipeline.ID,
			StageType:     stageType,
			StageOrder:    i + 1,
			Status:        types.StatusPending,
			RetryCount:    0,
		}

		if err := s.stageRepo.Create(ctx, stage); err != nil {
			logger.Error("创建阶段失败",
				zap.String("pipeline_id", pipeline.ID),
				zap.String("stage_type", string(stageType)),
				zap.Error(err),
			)
			return fmt.Errorf("创建阶段失败: %w", err)
		}
	}

	logger.Info("流水线创建成功",
		zap.String("pipeline_id", pipeline.ID),
		zap.String("project_id", pipeline.ProjectID),
	)

	return nil
}

// GetPipeline 获取流水线运行
func (s *pipelineService) GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error) {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		logger.Error("获取流水线失败",
			zap.String("pipeline_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取流水线失败: %w", err)
	}

	return pipeline, nil
}

// ListPipelines 列出项目的流水线运行
func (s *pipelineService) ListPipelines(ctx context.Context, projectID string, page, pageSize int) ([]*types.PipelineRun, int, error) {
	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// 获取流水线列表
	pipelines, err := s.pipelineRepo.List(ctx, projectID, pageSize, offset)
	if err != nil {
		logger.Error("获取流水线列表失败",
			zap.String("project_id", projectID),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取流水线列表失败: %w", err)
	}

	// 获取总数
	total, err := s.pipelineRepo.CountByProject(ctx, projectID)
	if err != nil {
		logger.Error("获取流水线总数失败",
			zap.String("project_id", projectID),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取流水线总数失败: %w", err)
	}

	return pipelines, total, nil
}

// StartPipeline 启动流水线
func (s *pipelineService) StartPipeline(ctx context.Context, id string) error {
	// 获取流水线
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", id)
	}

	// 检查状态
	if pipeline.Status != types.StatusPending {
		return fmt.Errorf("流水线状态不允许启动: %s", pipeline.Status)
	}

	// 更新状态为运行中
	now := time.Now()
	pipeline.Status = types.StatusRunning
	pipeline.StartedAt = &now
	pipeline.CurrentStage = 1

	if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
		return fmt.Errorf("启动流水线失败: %w", err)
	}

	logger.Info("流水线已启动",
		zap.String("pipeline_id", id),
	)

	return nil
}

// CancelPipeline 取消流水线
func (s *pipelineService) CancelPipeline(ctx context.Context, id string) error {
	// 获取流水线
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", id)
	}

	// 检查状态
	if pipeline.Status != types.StatusRunning && pipeline.Status != types.StatusPending {
		return fmt.Errorf("流水线状态不允许取消: %s", pipeline.Status)
	}

	// 更新状态为已取消
	if err := s.pipelineRepo.UpdateStatus(ctx, id, types.StatusCanceled, "用户取消"); err != nil {
		return fmt.Errorf("取消流水线失败: %w", err)
	}

	logger.Info("流水线已取消",
		zap.String("pipeline_id", id),
	)

	return nil
}

// AdvanceStage 推进到下一阶段
func (s *pipelineService) AdvanceStage(ctx context.Context, pipelineID string) error {
	// 获取流水线
	pipeline, err := s.pipelineRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", pipelineID)
	}

	// 检查是否还有下一阶段
	if pipeline.CurrentStage >= 6 {
		// 所有阶段完成，更新流水线状态为成功
		now := time.Now()
		pipeline.Status = types.StatusSucceeded
		pipeline.FinishedAt = &now

		if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
			return fmt.Errorf("更新流水线状态失败: %w", err)
		}

		logger.Info("流水线所有阶段完成",
			zap.String("pipeline_id", pipelineID),
		)

		return nil
	}

	// 推进到下一阶段
	pipeline.CurrentStage++

	if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
		return fmt.Errorf("推进阶段失败: %w", err)
	}

	logger.Info("流水线推进到下一阶段",
		zap.String("pipeline_id", pipelineID),
		zap.Int("current_stage", pipeline.CurrentStage),
	)

	return nil
}

// UpdatePipelineStatus 更新流水线状态
func (s *pipelineService) UpdatePipelineStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	if err := s.pipelineRepo.UpdateStatus(ctx, id, status, errorMsg); err != nil {
		return fmt.Errorf("更新流水线状态失败: %w", err)
	}

	logger.Info("流水线状态已更新",
		zap.String("pipeline_id", id),
		zap.String("status", string(status)),
	)

	return nil
}

// ListStages 列出流水线的所有阶段
func (s *pipelineService) ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error) {
	stages, err := s.stageRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		logger.Error("获取阶段列表失败",
			zap.String("pipeline_id", pipelineID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取阶段列表失败: %w", err)
	}

	return stages, nil
}

// CreateStage 创建阶段运行
func (s *pipelineService) CreateStage(ctx context.Context, stage *types.StageRun) error {
	if err := s.stageRepo.Create(ctx, stage); err != nil {
		logger.Error("创建阶段失败",
			zap.String("pipeline_id", stage.PipelineRunID),
			zap.String("stage_type", string(stage.StageType)),
			zap.Error(err),
		)
		return fmt.Errorf("创建阶段失败: %w", err)
	}

	return nil
}

// UpdateStage 更新阶段运行
func (s *pipelineService) UpdateStage(ctx context.Context, stage *types.StageRun) error {
	if err := s.stageRepo.Update(ctx, stage); err != nil {
		logger.Error("更新阶段失败",
			zap.String("stage_id", stage.ID),
			zap.Error(err),
		)
		return fmt.Errorf("更新阶段失败: %w", err)
	}

	return nil
}

// validatePipeline 验证流水线信息
func (s *pipelineService) validatePipeline(ctx context.Context, pipeline *types.PipelineRun) error {
	if pipeline.ProjectID == "" {
		return fmt.Errorf("项目ID不能为空")
	}

	if pipeline.DatasetID == "" {
		return fmt.Errorf("数据集ID不能为空")
	}

	// 检查项目是否存在
	_, err := s.projectRepo.GetByID(ctx, pipeline.ProjectID)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", pipeline.ProjectID)
	}

	// 检查数据集是否存在
	_, err = s.datasetRepo.GetByID(ctx, pipeline.DatasetID)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", pipeline.DatasetID)
	}

	// 验证训练配置
	if pipeline.TrainingConfig.NumTrainEpochs <= 0 {
		return fmt.Errorf("训练轮数必须大于0")
	}

	if pipeline.TrainingConfig.LearningRate <= 0 {
		return fmt.Errorf("学习率必须大于0")
	}

	// 验证资源请求
	if pipeline.ResourceRequest.GPUCount < 0 {
		return fmt.Errorf("GPU数量不能为负数")
	}

	return nil
}
