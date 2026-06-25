package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"go.uber.org/zap"
)

type PipelineService interface {
	CreatePipeline(ctx context.Context, pipeline *types.PipelineRun) error
	GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error)
	ListPipelines(ctx context.Context, uid int, projectID string, page, pageSize int) ([]*types.PipelineRun, int, error)
	StartPipeline(ctx context.Context, id string) error
	CancelPipeline(ctx context.Context, id string) error
	AdvanceStage(ctx context.Context, pipelineID string) error
	UpdatePipelineStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error)
	CreateStage(ctx context.Context, stage *types.StageRun) error
	UpdateStage(ctx context.Context, stage *types.StageRun) error
	GetStage(ctx context.Context, stageID string) (*types.StageRun, error)
}

type pipelineService struct {
	pipelineRepo mysqlrepo.PipelineRepository
	stageRepo    mysqlrepo.StageRepository
	projectRepo  mysqlrepo.ProjectRepository
	datasetRepo  mysqlrepo.DatasetRepository
	executorSvc  ExecutorService
}

func NewPipelineService(
	pipelineRepo mysqlrepo.PipelineRepository,
	stageRepo mysqlrepo.StageRepository,
	projectRepo mysqlrepo.ProjectRepository,
	datasetRepo mysqlrepo.DatasetRepository,
	executorSvc ExecutorService,
) PipelineService {
	return &pipelineService{
		pipelineRepo: pipelineRepo,
		stageRepo:    stageRepo,
		projectRepo:  projectRepo,
		datasetRepo:  datasetRepo,
		executorSvc:  executorSvc,
	}
}

func (s *pipelineService) CreatePipeline(ctx context.Context, pipeline *types.PipelineRun) error {
	if err := s.validatePipeline(ctx, pipeline); err != nil {
		return err
	}
	if pipeline.TriggerMode == "" {
		pipeline.TriggerMode = "manual"
	}
	pipeline.Status = types.StatusPending
	pipeline.CurrentStage = 0

	if err := s.pipelineRepo.Create(ctx, pipeline); err != nil {
		logger.Error("create pipeline failed",
			zap.String("project_id", pipeline.ProjectID),
			zap.Int("uid", pipeline.UID),
			zap.Error(err),
		)
		return fmt.Errorf("创建流水线失败: %w", err)
	}

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
			return fmt.Errorf("创建阶段失败: %w", err)
		}
	}

	logger.Info("pipeline created",
		zap.String("pipeline_id", pipeline.ID),
		zap.Int("uid", pipeline.UID),
		zap.String("project_id", pipeline.ProjectID),
	)
	return nil
}

func (s *pipelineService) GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error) {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取流水线失败: %w", err)
	}
	return pipeline, nil
}

func (s *pipelineService) ListPipelines(ctx context.Context, uid int, projectID string, page, pageSize int) ([]*types.PipelineRun, int, error) {
	if uid <= 0 {
		return nil, 0, fmt.Errorf("uid 必须大于0")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	pipelines, err := s.pipelineRepo.List(ctx, uid, projectID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("获取流水线列表失败: %w", err)
	}
	total, err := s.pipelineRepo.CountByProject(ctx, uid, projectID)
	if err != nil {
		return nil, 0, fmt.Errorf("获取流水线总数失败: %w", err)
	}
	return pipelines, total, nil
}

func (s *pipelineService) StartPipeline(ctx context.Context, id string) error {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", id)
	}
	if pipeline.Status != types.StatusPending {
		return fmt.Errorf("流水线状态不允许启动: %s", pipeline.Status)
	}

	now := time.Now()
	pipeline.Status = types.StatusRunning
	pipeline.StartedAt = &now
	pipeline.CurrentStage = 1

	if err := s.activateStageByOrder(ctx, id, 1, now); err != nil {
		return err
	}
	if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
		return fmt.Errorf("启动流水线失败: %w", err)
	}
	if err := s.executorSvc.SubmitPipeline(ctx, id); err != nil {
		pipeline.Status = types.StatusFailed
		pipeline.ErrorMessage = fmt.Sprintf("提交执行队列失败: %v", err)
		_ = s.pipelineRepo.Update(ctx, pipeline)
		return fmt.Errorf("提交执行队列失败: %w", err)
	}
	return nil
}

func (s *pipelineService) CancelPipeline(ctx context.Context, id string) error {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", id)
	}
	if pipeline.Status != types.StatusRunning && pipeline.Status != types.StatusPending {
		return fmt.Errorf("流水线状态不允许取消: %s", pipeline.Status)
	}

	now := time.Now()
	if pipeline.CurrentStage > 0 {
		if err := s.finishStageByOrder(ctx, id, pipeline.CurrentStage, types.StatusCanceled, now, "用户取消"); err != nil {
			return err
		}
	}
	if err := s.pipelineRepo.UpdateStatus(ctx, id, types.StatusCanceled, "用户取消"); err != nil {
		return fmt.Errorf("取消流水线失败: %w", err)
	}
	return nil
}

func (s *pipelineService) AdvanceStage(ctx context.Context, pipelineID string) error {
	pipeline, err := s.pipelineRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", pipelineID)
	}

	now := time.Now()
	if pipeline.CurrentStage > 0 {
		if err := s.finishStageByOrder(ctx, pipelineID, pipeline.CurrentStage, types.StatusSucceeded, now, ""); err != nil {
			return err
		}
	}

	if pipeline.CurrentStage >= 6 {
		pipeline.Status = types.StatusSucceeded
		pipeline.FinishedAt = &now
		if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
			return fmt.Errorf("更新流水线状态失败: %w", err)
		}
		return nil
	}

	pipeline.CurrentStage++
	if err := s.activateStageByOrder(ctx, pipelineID, pipeline.CurrentStage, now); err != nil {
		return err
	}
	if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
		return fmt.Errorf("推进阶段失败: %w", err)
	}
	return nil
}

func (s *pipelineService) UpdatePipelineStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	if err := s.pipelineRepo.UpdateStatus(ctx, id, status, errorMsg); err != nil {
		return fmt.Errorf("更新流水线状态失败: %w", err)
	}
	return nil
}

func (s *pipelineService) ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error) {
	stages, err := s.stageRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("获取阶段列表失败: %w", err)
	}
	return stages, nil
}

func (s *pipelineService) CreateStage(ctx context.Context, stage *types.StageRun) error {
	if err := s.stageRepo.Create(ctx, stage); err != nil {
		return fmt.Errorf("创建阶段失败: %w", err)
	}
	return nil
}

func (s *pipelineService) UpdateStage(ctx context.Context, stage *types.StageRun) error {
	if err := s.stageRepo.Update(ctx, stage); err != nil {
		return fmt.Errorf("更新阶段失败: %w", err)
	}
	return nil
}

func (s *pipelineService) GetStage(ctx context.Context, stageID string) (*types.StageRun, error) {
	stage, err := s.stageRepo.GetByID(ctx, stageID)
	if err != nil {
		return nil, fmt.Errorf("获取阶段失败: %w", err)
	}
	return stage, nil
}

func (s *pipelineService) activateStageByOrder(ctx context.Context, pipelineID string, stageOrder int, startedAt time.Time) error {
	stage, err := s.getStageByOrder(ctx, pipelineID, stageOrder)
	if err != nil {
		return err
	}
	stage.Status = types.StatusRunning
	stage.StartedAt = &startedAt
	stage.FinishedAt = nil
	stage.ErrorMessage = ""
	if err := s.stageRepo.Update(ctx, stage); err != nil {
		return fmt.Errorf("更新阶段失败: %w", err)
	}
	return nil
}

func (s *pipelineService) finishStageByOrder(ctx context.Context, pipelineID string, stageOrder int, status types.PipelineStatus, finishedAt time.Time, errorMsg string) error {
	stage, err := s.getStageByOrder(ctx, pipelineID, stageOrder)
	if err != nil {
		return err
	}
	stage.Status = status
	stage.FinishedAt = &finishedAt
	if stage.StartedAt == nil {
		stage.StartedAt = &finishedAt
	}
	stage.ErrorMessage = errorMsg
	if err := s.stageRepo.Update(ctx, stage); err != nil {
		return fmt.Errorf("更新阶段失败: %w", err)
	}
	return nil
}

func (s *pipelineService) getStageByOrder(ctx context.Context, pipelineID string, stageOrder int) (*types.StageRun, error) {
	stages, err := s.stageRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("获取阶段列表失败: %w", err)
	}
	for _, stage := range stages {
		if stage.StageOrder == stageOrder {
			return stage, nil
		}
	}
	return nil, fmt.Errorf("流水线缺少阶段 %d: %s", stageOrder, pipelineID)
}

func (s *pipelineService) validatePipeline(ctx context.Context, pipeline *types.PipelineRun) error {
	if pipeline.UID <= 0 {
		return newValidationError("uid 必须大于0")
	}
	if pipeline.ProjectID == "" {
		return newValidationError("项目ID不能为空")
	}
	if pipeline.DatasetID == "" {
		return newValidationError("数据集ID不能为空")
	}

	project, err := s.projectRepo.GetByID(ctx, pipeline.ProjectID)
	if err != nil {
		return newValidationError(fmt.Sprintf("项目不存在: %s", pipeline.ProjectID))
	}
	if project.UID != pipeline.UID {
		return newValidationError("流水线 uid 必须与项目 uid 一致")
	}

	dataset, err := s.datasetRepo.GetByID(ctx, pipeline.DatasetID)
	if err != nil {
		return newValidationError(fmt.Sprintf("数据集不存在: %s", pipeline.DatasetID))
	}
	if dataset.UID != pipeline.UID {
		return newValidationError("流水线 uid 必须与数据集 uid 一致")
	}

	if pipeline.TrainingConfig.NumTrainEpochs <= 0 {
		return newValidationError("训练轮数必须大于0")
	}
	if pipeline.TrainingConfig.LearningRate <= 0 {
		return newValidationError("学习率必须大于0")
	}
	if pipeline.ResourceRequest.GPUCount < 0 {
		return newValidationError("GPU数量不能为负数")
	}
	return nil
}
