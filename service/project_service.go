package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(ctx context.Context, project *types.Project) error
	GetProject(ctx context.Context, uid int, id string) (*types.Project, error)
	ListProjects(ctx context.Context, uid, page, pageSize int) ([]*types.Project, int, error)
	UpdateProject(ctx context.Context, project *types.Project) error
	DeleteProject(ctx context.Context, uid int, id string) error
}

type projectService struct {
	projectRepo mysqlrepo.ProjectRepository
	modelSvc    ModelService
}

func NewProjectService(projectRepo mysqlrepo.ProjectRepository, modelSvc ...ModelService) ProjectService {
	var svc ModelService
	if len(modelSvc) > 0 {
		svc = modelSvc[0]
	}
	return &projectService{
		projectRepo: projectRepo,
		modelSvc:    svc,
	}
}

func (s *projectService) CreateProject(ctx context.Context, project *types.Project) error {
	if err := s.prepareProject(ctx, project); err != nil {
		return err
	}

	if err := s.projectRepo.Create(ctx, project); err != nil {
		logger.Error("create distill project failed",
			zap.String("name", project.Name),
			zap.Int("uid", project.UID),
			zap.Error(err),
		)
		return fmt.Errorf("创建项目失败: %w", err)
	}

	logger.Info("distill project created",
		zap.String("project_id", project.ID),
		zap.Int("uid", project.UID),
		zap.String("name", project.Name),
	)
	return nil
}

func (s *projectService) GetProject(ctx context.Context, uid int, id string) (*types.Project, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("uid 必须大于0")
	}
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		logger.Error("get distill project failed",
			zap.String("project_id", id),
			zap.Int("uid", uid),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}
	if project.UID != uid {
		return nil, fmt.Errorf("项目不属于当前 uid: %s", id)
	}
	return project, nil
}

func (s *projectService) ListProjects(ctx context.Context, uid, page, pageSize int) ([]*types.Project, int, error) {
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
	projects, err := s.projectRepo.List(ctx, uid, pageSize, offset)
	if err != nil {
		logger.Error("list distill projects failed", zap.Int("uid", uid), zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目列表失败: %w", err)
	}

	total, err := s.projectRepo.Count(ctx, uid)
	if err != nil {
		logger.Error("count distill projects failed", zap.Int("uid", uid), zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目总数失败: %w", err)
	}
	return projects, total, nil
}

func (s *projectService) UpdateProject(ctx context.Context, project *types.Project) error {
	if err := s.prepareProject(ctx, project); err != nil {
		return err
	}

	existing, err := s.projectRepo.GetByID(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", project.ID)
	}
	if existing.UID != project.UID {
		return fmt.Errorf("项目不属于当前 uid: %s", project.ID)
	}

	if err := s.projectRepo.Update(ctx, project); err != nil {
		logger.Error("update distill project failed",
			zap.String("project_id", project.ID),
			zap.Int("uid", project.UID),
			zap.Error(err),
		)
		return fmt.Errorf("更新项目失败: %w", err)
	}
	return nil
}

func (s *projectService) DeleteProject(ctx context.Context, uid int, id string) error {
	if uid <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", id)
	}
	if project.UID != uid {
		return fmt.Errorf("项目不属于当前 uid: %s", id)
	}

	if err := s.projectRepo.Delete(ctx, id); err != nil {
		logger.Error("delete distill project failed",
			zap.String("project_id", id),
			zap.Int("uid", uid),
			zap.Error(err),
		)
		return fmt.Errorf("删除项目失败: %w", err)
	}
	return nil
}

func (s *projectService) prepareProject(ctx context.Context, project *types.Project) error {
	if err := s.resolveLocalModel(ctx, "教师", &project.TeacherModelConfig, func(ctx context.Context, modelID string) (*LocalModel, error) {
		return s.modelSvc.GetTeacherModel(ctx, modelID)
	}); err != nil {
		return err
	}
	if err := s.resolveLocalModel(ctx, "学生", &project.StudentModelConfig, func(ctx context.Context, modelID string) (*LocalModel, error) {
		return s.modelSvc.GetStudentModel(ctx, modelID)
	}); err != nil {
		return err
	}
	return s.validateProject(project)
}

func (s *projectService) resolveLocalModel(
	ctx context.Context,
	label string,
	cfg *types.ModelConfig,
	getModel func(context.Context, string) (*LocalModel, error),
) error {
	if cfg.ProviderType != types.ProviderLocal {
		return nil
	}

	if cfg.ModelID != "" {
		if s.modelSvc == nil {
			return fmt.Errorf("%s本地模型无法通过 model_id 解析，请配置模型服务", label)
		}
		model, err := getModel(ctx, cfg.ModelID)
		if err != nil {
			return fmt.Errorf("%s本地模型不存在或不可用: %w", label, err)
		}
		if model == nil {
			return fmt.Errorf("%s本地模型不存在或不可用", label)
		}
		cfg.ModelPath = model.Path
		if cfg.ModelName == "" {
			cfg.ModelName = model.Name
		}
		return nil
	}

	if cfg.ModelPath == "" {
		return fmt.Errorf("%s本地模型必须提供 model_id", label)
	}
	if s.modelSvc != nil {
		if err := s.modelSvc.ValidateLocalModel(ctx, cfg.ModelPath); err != nil {
			return fmt.Errorf("%s本地模型路径不可用: %w", label, err)
		}
	}
	if cfg.ModelName == "" {
		cfg.ModelName = filepath.Base(cfg.ModelPath)
	}
	return nil
}

func (s *projectService) validateProject(project *types.Project) error {
	if project.UID <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	if project.Name == "" {
		return fmt.Errorf("项目名称不能为空")
	}
	if len(project.Name) > 255 {
		return fmt.Errorf("项目名称长度不能超过255个字符")
	}

	if project.TeacherModelConfig.ProviderType != types.ProviderAPI &&
		project.TeacherModelConfig.ProviderType != types.ProviderLocal {
		return fmt.Errorf("无效的教师模型 provider_type: %s", project.TeacherModelConfig.ProviderType)
	}
	if project.TeacherModelConfig.ModelName == "" {
		return fmt.Errorf("教师模型名称不能为空")
	}
	if project.TeacherModelConfig.ProviderType == types.ProviderLocal &&
		project.TeacherModelConfig.ModelPath == "" {
		return fmt.Errorf("教师本地模型路径不能为空")
	}

	if project.StudentModelConfig.ProviderType != types.ProviderLocal {
		return fmt.Errorf("学生模型必须使用本地模型 (provider_type=local)")
	}
	if project.StudentModelConfig.ModelName == "" {
		return fmt.Errorf("学生模型名称不能为空")
	}
	if project.StudentModelConfig.ModelPath == "" {
		return fmt.Errorf("学生本地模型路径不能为空")
	}
	return nil
}
