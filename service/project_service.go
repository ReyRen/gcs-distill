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

// ProjectService 项目服务接口
type ProjectService interface {
	// CreateProject 创建项目
	CreateProject(ctx context.Context, project *types.Project) error
	// GetProject 获取项目
	GetProject(ctx context.Context, id string) (*types.Project, error)
	// ListProjects 列出项目
	ListProjects(ctx context.Context, page, pageSize int) ([]*types.Project, int, error)
	// UpdateProject 更新项目
	UpdateProject(ctx context.Context, project *types.Project) error
	// DeleteProject 删除项目
	DeleteProject(ctx context.Context, id string) error
}

// projectService 项目服务实现
type projectService struct {
	projectRepo mysqlrepo.ProjectRepository
	modelSvc    ModelService
}

// NewProjectService 创建项目服务
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

// CreateProject 创建项目
func (s *projectService) CreateProject(ctx context.Context, project *types.Project) error {
	if err := s.prepareProject(ctx, project); err != nil {
		return err
	}

	// 创建项目
	if err := s.projectRepo.Create(ctx, project); err != nil {
		logger.Error("创建项目失败",
			zap.String("name", project.Name),
			zap.Error(err),
		)
		return fmt.Errorf("创建项目失败: %w", err)
	}

	logger.Info("项目创建成功",
		zap.String("project_id", project.ID),
		zap.String("name", project.Name),
	)

	return nil
}

// GetProject 获取项目
func (s *projectService) GetProject(ctx context.Context, id string) (*types.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		logger.Error("获取项目失败",
			zap.String("project_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}

	return project, nil
}

// ListProjects 列出项目
func (s *projectService) ListProjects(ctx context.Context, page, pageSize int) ([]*types.Project, int, error) {
	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// 获取项目列表
	projects, err := s.projectRepo.List(ctx, pageSize, offset)
	if err != nil {
		logger.Error("获取项目列表失败", zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目列表失败: %w", err)
	}

	// 获取总数
	total, err := s.projectRepo.Count(ctx)
	if err != nil {
		logger.Error("获取项目总数失败", zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目总数失败: %w", err)
	}

	return projects, total, nil
}

// UpdateProject 更新项目
func (s *projectService) UpdateProject(ctx context.Context, project *types.Project) error {
	if err := s.prepareProject(ctx, project); err != nil {
		return err
	}

	// 检查项目是否存在
	_, err := s.projectRepo.GetByID(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", project.ID)
	}

	// 更新项目
	if err := s.projectRepo.Update(ctx, project); err != nil {
		logger.Error("更新项目失败",
			zap.String("project_id", project.ID),
			zap.Error(err),
		)
		return fmt.Errorf("更新项目失败: %w", err)
	}

	logger.Info("项目更新成功",
		zap.String("project_id", project.ID),
		zap.String("name", project.Name),
	)

	return nil
}

// DeleteProject 删除项目
func (s *projectService) DeleteProject(ctx context.Context, id string) error {
	// 检查项目是否存在
	_, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", id)
	}

	// 删除项目（级联删除相关数据集和流水线）
	if err := s.projectRepo.Delete(ctx, id); err != nil {
		logger.Error("删除项目失败",
			zap.String("project_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除项目失败: %w", err)
	}

	logger.Info("项目删除成功", zap.String("project_id", id))

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

// validateProject 验证项目信息
func (s *projectService) validateProject(project *types.Project) error {
	if project.Name == "" {
		return fmt.Errorf("项目名称不能为空")
	}

	if len(project.Name) > 255 {
		return fmt.Errorf("项目名称长度不能超过255个字符")
	}

	if project.TeacherModelConfig.ProviderType != types.ProviderAPI &&
		project.TeacherModelConfig.ProviderType != types.ProviderLocal {
		return fmt.Errorf("无效的教师模型提供者类型: %s", project.TeacherModelConfig.ProviderType)
	}

	if project.TeacherModelConfig.ModelName == "" {
		return fmt.Errorf("教师模型名称不能为空")
	}
	if project.TeacherModelConfig.ProviderType == types.ProviderLocal &&
		project.TeacherModelConfig.ModelPath == "" {
		return fmt.Errorf("教师本地模型路径不能为空")
	}

	// 学生模型必须是本地模型（离线环境）
	if project.StudentModelConfig.ProviderType != types.ProviderLocal {
		return fmt.Errorf("学生模型必须使用本地模型 (provider_type=local)，当前环境不支持在线模型")
	}

	if project.StudentModelConfig.ModelName == "" {
		return fmt.Errorf("学生模型名称不能为空")
	}
	if project.StudentModelConfig.ModelPath == "" {
		return fmt.Errorf("学生本地模型路径不能为空")
	}

	return nil
}
