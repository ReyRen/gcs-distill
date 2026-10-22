package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"github.com/ReyRen/gcs-distill/runtime"
	"go.uber.org/zap"
)

type PipelineService interface {
	CreatePipeline(ctx context.Context, pipeline *types.PipelineRun) error
	GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error)
	ListPipelines(ctx context.Context, uid int, projectID string, page, pageSize int, filters ...types.ListFilter) ([]*types.PipelineRun, int, error)
	StartPipeline(ctx context.Context, id string) error
	CancelPipeline(ctx context.Context, id string) error
	ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error)
	GetStage(ctx context.Context, stageID string) (*types.StageRun, error)
	ValidatePipeline(ctx context.Context, pipeline *types.PipelineRun) error
}

type pipelineService struct {
	pipelineRepo mysqlrepo.PipelineRepository
	stageRepo    mysqlrepo.StageRepository
	projectRepo  mysqlrepo.ProjectRepository
	datasetRepo  mysqlrepo.DatasetRepository
	executorSvc  ExecutorService
	modelSvc     ModelService
	teachers     *TeacherService
	resources    *ResourceService
	trainerImage string
}

type PipelineOptions struct {
	Resources    *ResourceService
	TrainerImage string
}

func NewPipelineService(
	pipelineRepo mysqlrepo.PipelineRepository,
	stageRepo mysqlrepo.StageRepository,
	projectRepo mysqlrepo.ProjectRepository,
	datasetRepo mysqlrepo.DatasetRepository,
	executorSvc ExecutorService,
	modelSvc ModelService,
	teachers *TeacherService,
	options PipelineOptions,
) PipelineService {
	return &pipelineService{
		pipelineRepo: pipelineRepo,
		stageRepo:    stageRepo,
		projectRepo:  projectRepo,
		datasetRepo:  datasetRepo,
		executorSvc:  executorSvc,
		modelSvc:     modelSvc,
		teachers:     teachers,
		resources:    options.Resources,
		trainerImage: options.TrainerImage,
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

	stages := runtime.PipelineStageTypes(pipeline)
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
	s.enrichPipeline(ctx, pipeline)
	return nil
}

func (s *pipelineService) GetPipeline(ctx context.Context, id string) (*types.PipelineRun, error) {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取流水线失败: %w", err)
	}
	s.enrichPipeline(ctx, pipeline)
	return pipeline, nil
}

func (s *pipelineService) ListPipelines(ctx context.Context, uid int, projectID string, page, pageSize int, filters ...types.ListFilter) ([]*types.PipelineRun, int, error) {
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
	pipelines, err := s.pipelineRepo.List(ctx, uid, projectID, pageSize, offset, filters...)
	if err != nil {
		return nil, 0, fmt.Errorf("获取流水线列表失败: %w", err)
	}
	total, err := s.pipelineRepo.CountByProject(ctx, uid, projectID, filters...)
	if err != nil {
		return nil, 0, fmt.Errorf("获取流水线总数失败: %w", err)
	}
	for _, p := range pipelines {
		s.enrichPipeline(ctx, p)
	}
	return pipelines, total, nil
}

func (s *pipelineService) StartPipeline(ctx context.Context, id string) error {
	p, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return newValidationError("任务不存在")
	}
	if p.TrainingConfig.Distillation == nil {
		return newValidationError("旧版任务已停用，请创建 EasyDistill 2 任务")
	}
	if s.resources != nil {
		if err := s.resources.Validate(ctx, &p.ResourceRequest, p.TrainingConfig.Distillation.TrainStudent, false); err != nil {
			return newValidationError(err.Error())
		}
	}
	claimed, err := s.pipelineRepo.Transition(ctx, id, types.StatusPending, types.StatusScheduled)
	if err != nil {
		return err
	}
	if !claimed {
		return newValidationError("任务状态不允许启动")
	}
	if err := s.executorSvc.SubmitPipeline(ctx, id); err != nil {
		_ = s.pipelineRepo.UpdateStatus(ctx, id, types.StatusFailed, "提交队列失败")
		return err
	}
	return nil
}

func (s *pipelineService) CancelPipeline(ctx context.Context, id string) error {
	pipeline, err := s.pipelineRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("流水线不存在: %s", id)
	}
	if pipeline.Status != types.StatusRunning && pipeline.Status != types.StatusPending && pipeline.Status != types.StatusScheduled && pipeline.Status != types.StatusPreparing {
		return fmt.Errorf("流水线状态不允许取消: %s", pipeline.Status)
	}

	if err := s.pipelineRepo.UpdateStatus(ctx, id, types.StatusCanceled, "用户取消"); err != nil {
		return fmt.Errorf("取消流水线失败: %w", err)
	}
	stages, err := s.stageRepo.ListByPipeline(ctx, id)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, stage := range stages {
		if stage.Status == types.StatusRunning || stage.Status == types.StatusPreparing {
			stage.Status = types.StatusCanceled
			stage.FinishedAt = &now
			stage.ErrorMessage = "用户取消"
			_ = s.stageRepo.Update(ctx, stage)
		}
	}
	return s.executorSvc.CancelContainers(ctx, id)
}

func (s *pipelineService) ListStages(ctx context.Context, pipelineID string) ([]*types.StageRun, error) {
	stages, err := s.stageRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("获取阶段列表失败: %w", err)
	}
	return stages, nil
}

func (s *pipelineService) GetStage(ctx context.Context, stageID string) (*types.StageRun, error) {
	stage, err := s.stageRepo.GetByID(ctx, stageID)
	if err != nil {
		return nil, fmt.Errorf("获取阶段失败: %w", err)
	}
	return stage, nil
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
	if pipeline.TrainingConfig.Distillation == nil {
		pipeline.TrainingConfig.Distillation = &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}
	}
	w := pipeline.TrainingConfig.Distillation
	if w.TrainStudent && s.trainerImage != "" {
		if w.TrainerImage == "" {
			w.TrainerImage = s.trainerImage
		}
		if w.TrainerImage != s.trainerImage {
			return newValidationError("请选择运行环境目录中已准备好的离线训练镜像")
		}
	}
	if w.TeacherModel == nil {
		teacher := project.TeacherModelConfig
		w.TeacherModel = &teacher
	}
	if s.teachers != nil {
		if err := s.teachers.Resolve(ctx, pipeline.UID, w.TeacherModel); err != nil {
			return newValidationError(err.Error())
		}
	}

	dataset, err := s.datasetRepo.GetByID(ctx, pipeline.DatasetID)
	if err != nil {
		return newValidationError(fmt.Sprintf("数据集不存在: %s", pipeline.DatasetID))
	}
	if dataset.UID != pipeline.UID {
		return newValidationError("流水线 uid 必须与数据集 uid 一致")
	}
	if w := pipeline.TrainingConfig.Distillation; w != nil && w.TrainStudent && w.StudentModel != nil {
		if s.modelSvc == nil {
			return newValidationError("学生模型目录服务不可用")
		}
		student := w.StudentModel
		if student.ModelID == "" {
			return newValidationError("请从共享模型池选择学生基模 ID")
		}
		if student.ModelID != "" {
			model, err := s.modelSvc.GetStudentModel(ctx, student.ModelID)
			if err != nil {
				return newValidationError("学生基模不存在")
			}
			student.ModelName = model.Name
			student.ModelPath = model.Path
		}
		student.ProviderType = types.ProviderLocal
		if w.ModelTemplate == "" {
			_, _, w.ModelTemplate = modelCapabilities(student.ModelPath)
		}
		if err := s.modelSvc.ValidateStudentModel(ctx, student.ModelPath); err != nil {
			return newValidationError(err.Error())
		}
		if strings.HasPrefix(w.JobType, "mm_") {
			_, vision, _ := modelCapabilities(student.ModelPath)
			if !vision {
				return newValidationError("多模态训练必须选择支持视觉的学生基模")
			}
		}
	}
	if err := runtime.NormalizeDistillation(pipeline, project); err != nil {
		return newValidationError(err.Error())
	}
	recordCount, err := runtime.WalkRecords(dataset.FilePath, func(row map[string]interface{}) error {
		return runtime.MapSeedRecord(row, w)
	})
	if err != nil {
		return newValidationError("种子数据校验失败: " + err.Error())
	}
	if recordCount == 0 {
		return newValidationError("种子数据集不能为空")
	}

	if pipeline.TrainingConfig.Distillation.TrainStudent && pipeline.TrainingConfig.NumTrainEpochs <= 0 {
		return newValidationError("训练轮数必须大于0")
	}
	if pipeline.TrainingConfig.Distillation.TrainStudent && pipeline.TrainingConfig.LearningRate <= 0 {
		return newValidationError("学习率必须大于0")
	}
	if pipeline.TrainingConfig.Distillation.TrainStudent && pipeline.TrainingConfig.PerDeviceTrainBatchSize <= 0 {
		return newValidationError("每卡训练批量必须大于0")
	}
	if pipeline.ResourceRequest.GPUCount < 0 {
		return newValidationError("GPU数量不能为负数")
	}
	if s.resources != nil {
		if err := s.resources.Validate(ctx, &pipeline.ResourceRequest, w.TrainStudent, false); err != nil {
			return newValidationError(err.Error())
		}
	}
	return nil
}

func (s *pipelineService) ValidatePipeline(ctx context.Context, p *types.PipelineRun) error {
	return s.validatePipeline(ctx, p)
}
