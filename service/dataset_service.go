package service

import (
	"bufio"
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const defaultDatasetBaseRel = "infer-center/model-distill/datasets"

type DatasetCandidate struct {
	Name        string    `json:"name"`
	FilePath    string    `json:"file_path"`
	SourceDir   string    `json:"source_dir,omitempty"`
	IsDirectory bool      `json:"is_directory"`
	SizeBytes   int64     `json:"size_bytes"`
	UpdatedAt   time.Time `json:"updated_at"`
	RecordCount int       `json:"record_count"`
}

type DatasetService interface {
	CreateDataset(ctx context.Context, dataset *types.Dataset) error
	CreateUploadedDataset(ctx context.Context, dataset *types.Dataset, file multipart.File, originalFilename string) error
	GetDataset(ctx context.Context, id string) (*types.Dataset, error)
	ListDatasets(ctx context.Context, projectID string, page, pageSize int) ([]*types.Dataset, int, error)
	ListDatasetCandidates(ctx context.Context) ([]DatasetCandidate, error)
	UpdateDataset(ctx context.Context, dataset *types.Dataset) error
	DeleteDataset(ctx context.Context, id string) error
	GetDatasetPath(projectID, datasetID string) string
}

type datasetService struct {
	datasetRepo mysqlrepo.DatasetRepository
	projectRepo mysqlrepo.ProjectRepository
	storageCfg  *config.StorageConfig
}

func NewDatasetService(
	datasetRepo mysqlrepo.DatasetRepository,
	projectRepo mysqlrepo.ProjectRepository,
	storageCfg *config.StorageConfig,
) DatasetService {
	return &datasetService{
		datasetRepo: datasetRepo,
		projectRepo: projectRepo,
		storageCfg:  storageCfg,
	}
}

func (s *datasetService) CreateDataset(ctx context.Context, dataset *types.Dataset) error {
	if err := s.prepareDataset(ctx, dataset); err != nil {
		return err
	}

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

func (s *datasetService) CreateUploadedDataset(ctx context.Context, dataset *types.Dataset, file multipart.File, originalFilename string) error {
	if err := s.prepareDataset(ctx, dataset); err != nil {
		return err
	}

	safeFilename := filepath.Base(strings.ReplaceAll(strings.TrimSpace(originalFilename), "\\", "/"))
	if safeFilename == "." || safeFilename == string(filepath.Separator) || safeFilename == "" {
		return fmt.Errorf("上传文件名不能为空")
	}

	datasetDir := s.GetDatasetPath(dataset.ProjectID, dataset.ID)
	if err := os.MkdirAll(datasetDir, 0o755); err != nil {
		return fmt.Errorf("创建数据集目录失败: %w", err)
	}

	dataset.FilePath = filepath.Join(datasetDir, safeFilename)
	targetFile, err := os.Create(dataset.FilePath)
	if err != nil {
		return fmt.Errorf("创建数据集文件失败: %w", err)
	}

	copyErr := func() error {
		defer targetFile.Close()
		_, err := targetFile.ReadFrom(file)
		return err
	}()
	if copyErr != nil {
		_ = os.Remove(dataset.FilePath)
		return fmt.Errorf("保存上传文件失败: %w", copyErr)
	}

	recordCount, err := countDatasetRecords(dataset.FilePath)
	if err != nil {
		_ = os.Remove(dataset.FilePath)
		return fmt.Errorf("统计数据集记录数失败: %w", err)
	}
	dataset.RecordCount = recordCount

	if err := s.datasetRepo.Create(ctx, dataset); err != nil {
		_ = os.RemoveAll(datasetDir)
		logger.Error("创建上传数据集失败",
			zap.String("project_id", dataset.ProjectID),
			zap.String("file_path", dataset.FilePath),
			zap.Error(err),
		)
		return fmt.Errorf("创建数据集失败: %w", err)
	}

	logger.Info("上传数据集创建成功",
		zap.String("dataset_id", dataset.ID),
		zap.String("project_id", dataset.ProjectID),
		zap.String("file_path", dataset.FilePath),
		zap.Int("record_count", dataset.RecordCount),
	)

	return nil
}

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

func (s *datasetService) ListDatasets(ctx context.Context, projectID string, page, pageSize int) ([]*types.Dataset, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	datasets, err := s.datasetRepo.ListByProject(ctx, projectID, pageSize, offset)
	if err != nil {
		logger.Error("获取数据集列表失败",
			zap.String("project_id", projectID),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取数据集列表失败: %w", err)
	}

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

func (s *datasetService) ListDatasetCandidates(ctx context.Context) ([]DatasetCandidate, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	baseDir := s.datasetBasePath()
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return []DatasetCandidate{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("扫描数据集目录失败: %w", err)
	}

	items := make([]DatasetCandidate, 0, len(entries))
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		entryPath := filepath.Join(baseDir, entry.Name())
		if entry.IsDir() {
			nested, err := s.listDatasetCandidatesInDir(entryPath, entry.Name())
			if err != nil {
				logger.Warn("扫描数据集子目录失败", zap.String("path", entryPath), zap.Error(err))
				continue
			}
			items = append(items, nested...)
			continue
		}
		if !isSupportedDatasetFile(entry.Name()) {
			continue
		}
		item, err := s.datasetCandidateFromFile(entryPath, entry.Name(), "")
		if err != nil {
			logger.Warn("读取数据集候选失败", zap.String("path", entryPath), zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func (s *datasetService) listDatasetCandidatesInDir(dirPath, dirName string) ([]DatasetCandidate, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	items := make([]DatasetCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !isSupportedDatasetFile(entry.Name()) {
			continue
		}
		item, err := s.datasetCandidateFromFile(
			filepath.Join(dirPath, entry.Name()),
			filepath.Join(dirName, entry.Name()),
			dirPath,
		)
		if err != nil {
			logger.Warn("读取数据集候选失败", zap.String("path", filepath.Join(dirPath, entry.Name())), zap.Error(err))
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *datasetService) datasetCandidateFromFile(filePath, displayName, sourceDir string) (DatasetCandidate, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return DatasetCandidate{}, err
	}
	if info.IsDir() {
		return DatasetCandidate{}, fmt.Errorf("dataset candidate is a directory: %s", filePath)
	}

	recordCount, err := countDatasetRecords(filePath)
	if err != nil {
		recordCount = 0
	}

	cleanSourceDir := ""
	if sourceDir != "" {
		cleanSourceDir = filepath.Clean(sourceDir)
	}

	return DatasetCandidate{
		Name:        displayName,
		FilePath:    filepath.Clean(filePath),
		SourceDir:   cleanSourceDir,
		IsDirectory: false,
		SizeBytes:   info.Size(),
		UpdatedAt:   info.ModTime(),
		RecordCount: recordCount,
	}, nil
}

func (s *datasetService) UpdateDataset(ctx context.Context, dataset *types.Dataset) error {
	if dataset.Name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}

	if _, err := s.datasetRepo.GetByID(ctx, dataset.ID); err != nil {
		return fmt.Errorf("数据集不存在: %s", dataset.ID)
	}
	if dataset.SourceType == "import" {
		if err := s.ensureDatasetFilePath(dataset.FilePath); err != nil {
			return err
		}
	}

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

func (s *datasetService) DeleteDataset(ctx context.Context, id string) error {
	dataset, err := s.datasetRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", id)
	}

	if dataset.FilePath != "" && dataset.SourceType != "import" {
		if err := s.removeOwnedDatasetArtifact(dataset); err != nil {
			logger.Warn("删除数据集文件失败",
				zap.String("file_path", dataset.FilePath),
				zap.Error(err),
			)
		}
	}

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

func (s *datasetService) GetDatasetPath(_ string, datasetID string) string {
	return filepath.Join(s.datasetBasePath(), datasetID)
}

func (s *datasetService) prepareDataset(ctx context.Context, dataset *types.Dataset) error {
	if dataset.ID == "" {
		dataset.ID = uuid.New().String()
	}

	if dataset.Name == "" && dataset.FilePath != "" {
		dataset.Name = filepath.Base(dataset.FilePath)
	}

	if err := s.validateDataset(dataset); err != nil {
		return err
	}

	if _, err := s.projectRepo.GetByID(ctx, dataset.ProjectID); err != nil {
		return fmt.Errorf("项目不存在: %s", dataset.ProjectID)
	}

	if dataset.SourceType == "import" {
		if err := s.ensureDatasetFilePath(dataset.FilePath); err != nil {
			return err
		}
		if dataset.RecordCount <= 0 {
			recordCount, err := countDatasetRecords(dataset.FilePath)
			if err != nil {
				return fmt.Errorf("统计数据集记录数失败: %w", err)
			}
			dataset.RecordCount = recordCount
		}
	}

	return nil
}

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
	if dataset.SourceType == "import" && strings.TrimSpace(dataset.FilePath) == "" {
		return fmt.Errorf("导入数据集必须提供 file_path")
	}

	return nil
}

func (s *datasetService) datasetBasePath() string {
	if s.storageCfg == nil {
		return filepath.Clean(filepath.Join("/storage-root-jfs", filepath.FromSlash(defaultDatasetBaseRel)))
	}
	if path := strings.TrimSpace(s.storageCfg.DatasetsBasePath); path != "" {
		return filepath.Clean(path)
	}
	root := strings.TrimSpace(s.storageCfg.RootPath)
	if root == "" {
		root = "/storage-root-jfs"
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(defaultDatasetBaseRel)))
}

func (s *datasetService) ensureDatasetFilePath(filePath string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "." || cleanPath == "" {
		return fmt.Errorf("数据集 file_path 不能为空")
	}
	if err := s.ensurePathUnderDatasetBase(cleanPath); err != nil {
		return err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("数据集文件不可访问: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("数据集 file_path 必须指向具体文件")
	}
	if !isSupportedDatasetFile(cleanPath) {
		return fmt.Errorf("数据集文件类型不支持: %s", cleanPath)
	}
	return nil
}

func (s *datasetService) ensurePathUnderDatasetBase(path string) error {
	base := s.datasetBasePath()
	rel, err := filepath.Rel(base, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("数据集路径必须位于 %s 下", base)
	}
	return nil
}

func (s *datasetService) removeOwnedDatasetArtifact(dataset *types.Dataset) error {
	cleanPath := filepath.Clean(dataset.FilePath)
	if err := s.ensurePathUnderDatasetBase(cleanPath); err != nil {
		return err
	}

	parentDir := filepath.Dir(cleanPath)
	if filepath.Base(parentDir) == dataset.ID {
		return os.RemoveAll(parentDir)
	}
	return os.RemoveAll(cleanPath)
}

func isSupportedDatasetFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json", ".jsonl", ".ndjson", ".txt":
		return true
	default:
		return false
	}
}

func countDatasetRecords(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 1024*64)
	scanner.Buffer(buffer, 1024*1024)

	count := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if count == 0 {
		return 1, nil
	}
	return count, nil
}
