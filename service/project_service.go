package service

import (
	"context"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"go.uber.org/zap"
)

type ProjectService interface {
	CreateProject(ctx context.Context, project *types.Project) error
	GetProject(ctx context.Context, uid int, id string) (*types.Project, error)
	ListProjects(ctx context.Context, uid, page, pageSize int, filters ...types.ListFilter) ([]*types.Project, int, error)
	UpdateProject(ctx context.Context, project *types.Project) error
	DeleteProject(ctx context.Context, uid int, id string) error
}

type projectService struct {
	projectRepo mysqlrepo.ProjectRepository
}

func NewProjectService(repo mysqlrepo.ProjectRepository) ProjectService {
	return &projectService{projectRepo: repo}
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

func (s *projectService) ListProjects(ctx context.Context, uid, page, pageSize int, filters ...types.ListFilter) ([]*types.Project, int, error) {
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
	projects, err := s.projectRepo.List(ctx, uid, pageSize, offset, filters...)
	if err != nil {
		logger.Error("list distill projects failed", zap.Int("uid", uid), zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目列表失败: %w", err)
	}

	total, err := s.projectRepo.Count(ctx, uid, filters...)
	if err != nil {
		logger.Error("count distill projects failed", zap.Int("uid", uid), zap.Error(err))
		return nil, 0, fmt.Errorf("获取项目总数失败: %w", err)
	}
	return projects, total, nil
}

func (s *projectService) UpdateProject(ctx context.Context, project *types.Project) error {
	existing, err := s.projectRepo.GetByID(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", project.ID)
	}
	if existing.UID != project.UID {
		return fmt.Errorf("项目不属于当前 uid: %s", project.ID)
	}

	teacher := &project.TeacherModelConfig
	old := existing.TeacherModelConfig
	sameConnection := teacher.Endpoint == old.Endpoint && teacher.ModelName == old.ModelName && teacher.ServiceID == old.ServiceID
	if teacher.APISecretRef == "********" {
		if !sameConnection {
			return fmt.Errorf("教师连接已变更，请重新输入密钥")
		}
		teacher.APISecretRef = old.APISecretRef
	}
	for key, value := range teacher.ExtraParams {
		if value == "********" {
			if !sameConnection {
				return fmt.Errorf("教师连接已变更，请重新输入密钥")
			}
			teacher.ExtraParams[key] = old.ExtraParams[key]
		}
	}
	if err := s.prepareProject(ctx, project); err != nil {
		return err
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

func (s *projectService) prepareProject(_ context.Context, p *types.Project) error {
	if p.UID <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	if p.Name == "" || len([]rune(p.Name)) > 255 {
		return fmt.Errorf("项目名称必须为1至255个字符")
	}
	teacher := &p.TeacherModelConfig
	if teacher.ServiceID != "" {
		teacher.ProviderType = types.ProviderAPI
		return nil
	}
	if teacher.Endpoint == "" && teacher.ModelName == "" && teacher.ProviderType == "" {
		return nil
	}
	if teacher.ProviderType == types.ProviderLocal {
		return fmt.Errorf("本地教师请先部署为推理服务")
	}
	return validateTeacherEndpoint(teacher)
}
