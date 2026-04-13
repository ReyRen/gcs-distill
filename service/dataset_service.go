package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/repository/postgres"
	"go.uber.org/zap"
)

// DatasetService 数据集服务接口
type DatasetService interface {
	// CreateDataset 创建数据集
	CreateDataset(ctx context.Context, dataset *types.Dataset) error
	// GetDataset 获取数据集
	GetDataset(ctx context.Context, id string) (*types.Dataset, error)
	// ListDatasets 列出项目的数据集
	ListDatasets(ctx context.Context, projectID string, page, pageSize int) ([]*types.Dataset, int, error)
	// UpdateDataset 更新数据集
	UpdateDataset(ctx context.Context, dataset *types.Dataset) error
	// DeleteDataset 删除数据集
	DeleteDataset(ctx context.Context, id string) error
	// GetDatasetPath 获取数据集存储路径
	GetDatasetPath(projectID, datasetID string) string
}

// datasetService 数据集服务实现
type datasetService struct {
	datasetRepo postgres.DatasetRepository
	projectRepo postgres.ProjectRepository
	storageCfg  *config.StorageConfig
}

// NewDatasetService 创建数据集服务
func NewDatasetService(
	datasetRepo postgres.DatasetRepository,
	projectRepo postgres.ProjectRepository,
	storageCfg *config.StorageConfig,
) DatasetService {
	return &datasetService{
		datasetRepo: datasetRepo,
		projectRepo: projectRepo,
		storageCfg:  storageCfg,
	}
}

// CreateDataset 创建数据集
func (s *datasetService) CreateDataset(ctx context.Context, dataset *types.Dataset) error {
	// 验证数据集信息
	if err := s.validateDataset(dataset); err != nil {
		return err
	}

	// 检查项目是否存在
	_, err := s.projectRepo.GetByID(ctx, dataset.ProjectID)
	if err != nil {
		return fmt.Errorf("项目不存在: %s", dataset.ProjectID)
	}

	// 创建存储目录
	datasetPath := s.GetDatasetPath(dataset.ProjectID, dataset.ID)
	if err := os.MkdirAll(filepath.Dir(datasetPath), 0755); err != nil {
		return fmt.Errorf("创建数据集目录失败: %w", err)
	}

	// 创建数据集
	if err := s.datasetRepo.Create(ctx, dataset); err != nil {
		logger.Error("创建数据集失败",
			zap.String("project_id", dataset.ProjectID),
			zap.String("name", dataset.Name),
			zap.Error(err),
		)
		return fmt.Errorf("创建数据集失败: %w", err)
	}

	logger.Info("数据集创建成功",
		zap.String("dataset_id", dataset.ID),
		zap.String("project_id", dataset.ProjectID),
		zap.String("name", dataset.Name),
	)

	return nil
}

// GetDataset 获取数据集
func (s *datasetService) GetDataset(ctx context.Context, id string) (*types.Dataset, error) {
	dataset, err := s.datasetRepo.GetByID(ctx, id)
	if err != nil {
		logger.Error("获取数据集失败",
			zap.String("dataset_id", id),
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取数据集失败: %w", err)
	}

	return dataset, nil
}

// ListDatasets 列出项目的数据集
func (s *datasetService) ListDatasets(ctx context.Context, projectID string, page, pageSize int) ([]*types.Dataset, int, error) {
	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// 获取数据集列表
	datasets, err := s.datasetRepo.ListByProject(ctx, projectID, pageSize, offset)
	if err != nil {
		logger.Error("获取数据集列表失败",
			zap.String("project_id", projectID),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取数据集列表失败: %w", err)
	}

	// 获取总数
	total, err := s.datasetRepo.CountByProject(ctx, projectID)
	if err != nil {
		logger.Error("获取数据集总数失败",
			zap.String("project_id", projectID),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取数据集总数失败: %w", err)
	}

	return datasets, total, nil
}

// UpdateDataset 更新数据集
func (s *datasetService) UpdateDataset(ctx context.Context, dataset *types.Dataset) error {
	// 验证数据集信息
	if dataset.Name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}

	// 检查数据集是否存在
	_, err := s.datasetRepo.GetByID(ctx, dataset.ID)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", dataset.ID)
	}

	// 更新数据集
	if err := s.datasetRepo.Update(ctx, dataset); err != nil {
		logger.Error("更新数据集失败",
			zap.String("dataset_id", dataset.ID),
			zap.Error(err),
		)
		return fmt.Errorf("更新数据集失败: %w", err)
	}

	logger.Info("数据集更新成功",
		zap.String("dataset_id", dataset.ID),
		zap.String("name", dataset.Name),
	)

	return nil
}

// DeleteDataset 删除数据集
func (s *datasetService) DeleteDataset(ctx context.Context, id string) error {
	// 获取数据集信息
	dataset, err := s.datasetRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", id)
	}

	// 删除数据集文件
	if dataset.FilePath != "" {
		if err := os.RemoveAll(dataset.FilePath); err != nil {
			logger.Warn("删除数据集文件失败",
				zap.String("file_path", dataset.FilePath),
				zap.Error(err),
			)
		}
	}

	// 删除数据集记录
	if err := s.datasetRepo.Delete(ctx, id); err != nil {
		logger.Error("删除数据集失败",
			zap.String("dataset_id", id),
			zap.Error(err),
		)
		return fmt.Errorf("删除数据集失败: %w", err)
	}

	logger.Info("数据集删除成功", zap.String("dataset_id", id))

	return nil
}

// GetDatasetPath 获取数据集存储路径
func (s *datasetService) GetDatasetPath(projectID, datasetID string) string {
	// /shared/distill/projects/{project_id}/datasets/{dataset_id}/
	return filepath.Join(s.storageCfg.BasePath, "projects", projectID, "datasets", datasetID)
}

// validateDataset 验证数据集信息
func (s *datasetService) validateDataset(dataset *types.Dataset) error {
	if dataset.Name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}

	if len(dataset.Name) > 255 {
		return fmt.Errorf("数据集名称长度不能超过255个字符")
	}

	if dataset.ProjectID == "" {
		return fmt.Errorf("项目ID不能为空")
	}

	if dataset.SourceType == "" {
		return fmt.Errorf("数据来源类型不能为空")
	}

	validSourceTypes := map[string]bool{
		"upload":    true,
		"import":    true,
		"generated": true,
	}
	if !validSourceTypes[dataset.SourceType] {
		return fmt.Errorf("无效的数据来源类型: %s", dataset.SourceType)
	}

	return nil
}
