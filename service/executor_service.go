package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/repository/postgres"
	"github.com/ReyRen/gcs-distill/runtime"
	"go.uber.org/zap"
)

// ExecutorService 流水线执行服务
type ExecutorService interface {
	// SubmitPipeline 提交流水线到执行队列
	SubmitPipeline(ctx context.Context, pipelineID string) error
	// Start 启动后台执行器
	Start(ctx context.Context)
	// Stop 停止后台执行器
	Stop()
}

// executorService 流水线执行服务实现
type executorService struct {
	pipelineRepo  postgres.PipelineRepository
	stageRepo     postgres.StageRepository
	projectRepo   postgres.ProjectRepository
	schedulerSvc  SchedulerService
	stageExecutor *runtime.StageExecutor

	// 执行队列
	queue chan string
	// 停止信号
	stopChan chan struct{}
	// 等待组
	wg sync.WaitGroup
	// 并发控制
	maxConcurrent int
}

// NewExecutorService 创建流水线执行服务
func NewExecutorService(
	pipelineRepo postgres.PipelineRepository,
	stageRepo postgres.StageRepository,
	projectRepo postgres.ProjectRepository,
	schedulerSvc SchedulerService,
	workspaceRoot string,
	maxConcurrent int,
) ExecutorService {
	if maxConcurrent <= 0 {
		maxConcurrent = 5 // 默认最多并发执行5个流水线
	}

	return &executorService{
		pipelineRepo:  pipelineRepo,
		stageRepo:     stageRepo,
		projectRepo:   projectRepo,
		schedulerSvc:  schedulerSvc,
		stageExecutor: runtime.NewStageExecutor(workspaceRoot),
		queue:         make(chan string, 100),
		stopChan:      make(chan struct{}),
		maxConcurrent: maxConcurrent,
	}
}

// SubmitPipeline 提交流水线到执行队列
func (s *executorService) SubmitPipeline(ctx context.Context, pipelineID string) error {
	select {
	case s.queue <- pipelineID:
		logger.Info("流水线已提交到执行队列",
			zap.String("pipeline_id", pipelineID),
		)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("提交流水线超时: %w", ctx.Err())
	}
}

// Start 启动后台执行器
func (s *executorService) Start(ctx context.Context) {
	logger.Info("启动流水线执行器",
		zap.Int("max_concurrent", s.maxConcurrent),
	)

	// 启动多个工作协程
	for i := 0; i < s.maxConcurrent; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}
}

// Stop 停止后台执行器
func (s *executorService) Stop() {
	logger.Info("停止流水线执行器")
	close(s.stopChan)
	s.wg.Wait()
	logger.Info("流水线执行器已停止")
}

// worker 工作协程
func (s *executorService) worker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	logger.Info("工作协程启动", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.stopChan:
			logger.Info("工作协程退出", zap.Int("worker_id", workerID))
			return
		case pipelineID := <-s.queue:
			logger.Info("开始执行流水线",
				zap.Int("worker_id", workerID),
				zap.String("pipeline_id", pipelineID),
			)

			// 执行流水线
			if err := s.executePipeline(ctx, pipelineID); err != nil {
				logger.Error("流水线执行失败",
					zap.Int("worker_id", workerID),
					zap.String("pipeline_id", pipelineID),
					zap.Error(err),
				)
			}
		}
	}
}

// executePipeline 执行完整流水线
func (s *executorService) executePipeline(ctx context.Context, pipelineID string) error {
	// 获取流水线信息
	pipeline, err := s.pipelineRepo.GetByID(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("获取流水线失败: %w", err)
	}

	// 获取项目信息
	project, err := s.projectRepo.GetByID(ctx, pipeline.ProjectID)
	if err != nil {
		return fmt.Errorf("获取项目失败: %w", err)
	}

	// 查找可用的 Worker 节点
	node, err := s.schedulerSvc.FindAvailableNode(ctx, pipeline.ResourceRequest)
	if err != nil {
		// 更新流水线状态为失败
		_ = s.pipelineRepo.UpdateStatus(ctx, pipelineID, types.StatusFailed,
			fmt.Sprintf("无可用 Worker 节点: %v", err))
		return fmt.Errorf("查找可用节点失败: %w", err)
	}

	logger.Info("找到可用 Worker 节点",
		zap.String("pipeline_id", pipelineID),
		zap.String("node_name", node.NodeName),
		zap.String("node_addr", node.NodeAddr),
	)

	// 分配资源
	if err := s.schedulerSvc.AllocateResources(ctx, node.NodeName, pipeline.ResourceRequest); err != nil {
		_ = s.pipelineRepo.UpdateStatus(ctx, pipelineID, types.StatusFailed,
			fmt.Sprintf("资源分配失败: %v", err))
		return fmt.Errorf("分配资源失败: %w", err)
	}

	// 确保在退出时释放资源
	defer func() {
		if err := s.schedulerSvc.ReleaseResources(ctx, node.NodeName, pipeline.ResourceRequest); err != nil {
			logger.Error("释放资源失败",
				zap.String("pipeline_id", pipelineID),
				zap.String("node_name", node.NodeName),
				zap.Error(err),
			)
		}
	}()

	// 获取所有阶段
	stages, err := s.stageRepo.ListByPipeline(ctx, pipelineID)
	if err != nil {
		return fmt.Errorf("获取阶段列表失败: %w", err)
	}

	// 按阶段顺序执行
	for _, stage := range stages {
		if stage.StageOrder != pipeline.CurrentStage {
			continue
		}

		logger.Info("开始执行阶段",
			zap.String("pipeline_id", pipelineID),
			zap.String("stage_type", string(stage.StageType)),
			zap.Int("stage_order", stage.StageOrder),
		)

		// 更新阶段状态为运行中
		now := time.Now()
		stage.Status = types.StatusRunning
		stage.StartedAt = &now
		stage.NodeName = node.NodeName
		if err := s.stageRepo.Update(ctx, stage); err != nil {
			logger.Error("更新阶段状态失败", zap.Error(err))
		}

		// 执行阶段
		err := s.stageExecutor.ExecuteStage(ctx, stage, pipeline, project, node.NodeAddr)

		finishTime := time.Now()
		stage.FinishedAt = &finishTime

		if err != nil {
			// 阶段执行失败
			stage.Status = types.StatusFailed
			stage.ErrorMessage = err.Error()
			_ = s.stageRepo.Update(ctx, stage)

			// 更新流水线状态为失败
			_ = s.pipelineRepo.UpdateStatus(ctx, pipelineID, types.StatusFailed,
				fmt.Sprintf("阶段 %s 执行失败: %v", stage.StageType, err))

			logger.Error("阶段执行失败",
				zap.String("pipeline_id", pipelineID),
				zap.String("stage_type", string(stage.StageType)),
				zap.Error(err),
			)

			return fmt.Errorf("阶段 %s 执行失败: %w", stage.StageType, err)
		}

		// 阶段执行成功
		stage.Status = types.StatusSucceeded
		if err := s.stageRepo.Update(ctx, stage); err != nil {
			logger.Error("更新阶段状态失败", zap.Error(err))
		}

		logger.Info("阶段执行成功",
			zap.String("pipeline_id", pipelineID),
			zap.String("stage_type", string(stage.StageType)),
			zap.Int("stage_order", stage.StageOrder),
		)

		// 推进到下一阶段
		if stage.StageOrder < 6 {
			pipeline.CurrentStage++
			if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
				logger.Error("更新流水线阶段失败", zap.Error(err))
			}

			// 激活下一阶段
			nextStageTime := time.Now()
			for _, nextStage := range stages {
				if nextStage.StageOrder == pipeline.CurrentStage {
					nextStage.Status = types.StatusRunning
					nextStage.StartedAt = &nextStageTime
					if err := s.stageRepo.Update(ctx, nextStage); err != nil {
						logger.Error("激活下一阶段失败", zap.Error(err))
					}
					break
				}
			}
		}
	}

	// 所有阶段完成，更新流水线状态为成功
	finishTime := time.Now()
	pipeline.Status = types.StatusSucceeded
	pipeline.FinishedAt = &finishTime
	if err := s.pipelineRepo.Update(ctx, pipeline); err != nil {
		logger.Error("更新流水线状态失败", zap.Error(err))
		return fmt.Errorf("更新流水线状态失败: %w", err)
	}

	logger.Info("流水线执行完成",
		zap.String("pipeline_id", pipelineID),
		zap.Duration("duration", finishTime.Sub(*pipeline.StartedAt)),
	)

	return nil
}
