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
	GetDataset(ctx context.Context, uid int, id string) (*types.Dataset, error)
	ListDatasets(ctx context.Context, uid, page, pageSize int) ([]*types.Dataset, int, error)
	ListDatasetCandidates(ctx context.Context, uid int) ([]DatasetCandidate, error)
	UpdateDataset(ctx context.Context, dataset *types.Dataset) error
	DeleteDataset(ctx context.Context, uid int, id string) error
	GetDatasetPath(uid int, datasetID string) (string, error)
}

type datasetService struct {
	datasetRepo mysqlrepo.DatasetRepository
	storageCfg  *config.StorageConfig
}

func NewDatasetService(
	datasetRepo mysqlrepo.DatasetRepository,
	storageCfg *config.StorageConfig,
) DatasetService {
	return &datasetService{
		datasetRepo: datasetRepo,
		storageCfg:  storageCfg,
	}
}

func (s *datasetService) CreateDataset(ctx context.Context, dataset *types.Dataset) error {
	if dataset.SourceType != "import" {
		return fmt.Errorf("POST /api/v1/datasets 只用于登记候选数据集，上传文件请调用 POST /api/v1/datasets/upload")
	}
	if err := s.prepareDataset(dataset); err != nil {
		return err
	}

	if err := s.datasetRepo.Create(ctx, dataset); err != nil {
		logger.Error("create dataset failed",
			zap.String("name", dataset.Name),
			zap.Int("uid", dataset.UID),
			zap.Error(err),
		)
		return fmt.Errorf("创建数据集失败: %w", err)
	}
	return nil
}

func (s *datasetService) CreateUploadedDataset(ctx context.Context, dataset *types.Dataset, file multipart.File, originalFilename string) error {
	dataset.SourceType = "upload"
	if err := s.prepareDataset(dataset); err != nil {
		return err
	}

	safeFilename := filepath.Base(strings.ReplaceAll(strings.TrimSpace(originalFilename), "\\", "/"))
	if safeFilename == "." || safeFilename == string(filepath.Separator) || safeFilename == "" {
		return fmt.Errorf("上传文件名不能为空")
	}

	datasetDir, err := s.GetDatasetPath(dataset.UID, dataset.ID)
	if err != nil {
		return err
	}
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
	fillDatasetName(dataset)

	if err := s.datasetRepo.Create(ctx, dataset); err != nil {
		_ = os.RemoveAll(datasetDir)
		logger.Error("create uploaded dataset failed",
			zap.String("file_path", dataset.FilePath),
			zap.Int("uid", dataset.UID),
			zap.Error(err),
		)
		return fmt.Errorf("创建数据集失败: %w", err)
	}
	return nil
}

func (s *datasetService) GetDataset(ctx context.Context, uid int, id string) (*types.Dataset, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("uid 必须大于0")
	}
	dataset, err := s.datasetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取数据集失败: %w", err)
	}
	if dataset.UID != uid {
		return nil, fmt.Errorf("数据集不属于当前 uid: %s", id)
	}
	fillDatasetName(dataset)
	return dataset, nil
}

func (s *datasetService) ListDatasets(ctx context.Context, uid, page, pageSize int) ([]*types.Dataset, int, error) {
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
	datasets, err := s.datasetRepo.List(ctx, uid, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("获取数据集列表失败: %w", err)
	}
	fillDatasetNames(datasets)

	total, err := s.datasetRepo.Count(ctx, uid)
	if err != nil {
		return nil, 0, fmt.Errorf("获取数据集总数失败: %w", err)
	}
	return datasets, total, nil
}

func (s *datasetService) ListDatasetCandidates(ctx context.Context, uid int) ([]DatasetCandidate, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("uid 必须大于0")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	baseDir, err := s.datasetCandidatesPath(uid)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(baseDir)
	if os.IsNotExist(err) {
		return []DatasetCandidate{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("扫描数据集候选目录失败: %w", err)
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
				logger.Warn("scan dataset candidate subdir failed", zap.String("path", entryPath), zap.Error(err))
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
			logger.Warn("read dataset candidate failed", zap.String("path", entryPath), zap.Error(err))
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
			logger.Warn("read dataset candidate failed", zap.String("path", filepath.Join(dirPath, entry.Name())), zap.Error(err))
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
	if dataset.UID <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	existing, err := s.datasetRepo.GetByID(ctx, dataset.ID)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", dataset.ID)
	}
	if existing.UID != dataset.UID {
		return fmt.Errorf("数据集不属于当前 uid: %s", dataset.ID)
	}

	dataset.SourceType = existing.SourceType
	dataset.FilePath = existing.FilePath
	dataset.RecordCount = existing.RecordCount
	dataset.CreatedAt = existing.CreatedAt
	if err := s.validateDataset(dataset); err != nil {
		return err
	}
	fillDatasetName(dataset)

	if err := s.datasetRepo.Update(ctx, dataset); err != nil {
		return fmt.Errorf("更新数据集失败: %w", err)
	}
	return nil
}

func (s *datasetService) DeleteDataset(ctx context.Context, uid int, id string) error {
	if uid <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	dataset, err := s.datasetRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("数据集不存在: %s", id)
	}
	if dataset.UID != uid {
		return fmt.Errorf("数据集不属于当前 uid: %s", id)
	}

	if dataset.FilePath != "" && dataset.SourceType != "import" {
		if err := s.removeOwnedDatasetArtifact(dataset); err != nil {
			logger.Warn("remove dataset artifact failed",
				zap.String("file_path", dataset.FilePath),
				zap.Error(err),
			)
		}
	}

	if err := s.datasetRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除数据集失败: %w", err)
	}
	return nil
}

func (s *datasetService) GetDatasetPath(uid int, datasetID string) (string, error) {
	uploadsPath, err := s.datasetUploadsPath(uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(uploadsPath, datasetID), nil
}

func (s *datasetService) prepareDataset(dataset *types.Dataset) error {
	if dataset.ID == "" {
		dataset.ID = uuid.New().String()
	}

	dataset.Name = strings.TrimSpace(dataset.Name)
	dataset.Description = strings.TrimSpace(dataset.Description)
	if dataset.Name == "" && dataset.FilePath != "" {
		dataset.Name = filepath.Base(dataset.FilePath)
	}

	if err := s.validateDataset(dataset); err != nil {
		return err
	}
	fillDatasetName(dataset)

	if dataset.SourceType == "import" {
		if err := s.ensureDatasetFilePath(dataset.UID, dataset.FilePath); err != nil {
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
	fillDatasetName(dataset)
	return nil
}

func (s *datasetService) validateDataset(dataset *types.Dataset) error {
	if dataset.UID <= 0 {
		return fmt.Errorf("uid 必须大于0")
	}
	if dataset.Name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}
	if len(dataset.Name) > 255 {
		return fmt.Errorf("数据集名称长度不能超过255个字符")
	}
	if dataset.SourceType == "" {
		return fmt.Errorf("数据来源类型不能为空")
	}

	validSourceTypes := map[string]bool{
		"upload": true,
		"import": true,
	}
	if !validSourceTypes[dataset.SourceType] {
		return fmt.Errorf("无效的数据来源类型: %s", dataset.SourceType)
	}
	if dataset.SourceType == "import" && strings.TrimSpace(dataset.FilePath) == "" {
		return fmt.Errorf("导入数据集必须提供 file_path")
	}
	return nil
}

func (s *datasetService) storage() config.StorageConfig {
	if s.storageCfg == nil {
		return config.StorageConfig{RootPath: "/storage-root-jfs"}
	}
	return *s.storageCfg
}

func (s *datasetService) datasetBasePath(uid int) (string, error) {
	return s.storage().UserDatasetsBase(uid)
}

func (s *datasetService) datasetCandidatesPath(uid int) (string, error) {
	return s.storage().UserDatasetCandidates(uid)
}

func (s *datasetService) datasetUploadsPath(uid int) (string, error) {
	return s.storage().UserDatasetUploads(uid)
}

func (s *datasetService) ensureDatasetFilePath(uid int, filePath string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "." || cleanPath == "" {
		return fmt.Errorf("数据集 file_path 不能为空")
	}
	base, err := s.datasetCandidatesPath(uid)
	if err != nil {
		return err
	}
	if err := ensurePathUnderBase(cleanPath, base); err != nil {
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

func (s *datasetService) ensurePathUnderDatasetBase(dataset *types.Dataset, path string) error {
	base, err := s.datasetBasePath(dataset.UID)
	if err != nil {
		return err
	}
	return ensurePathUnderBase(path, base)
}

func ensurePathUnderBase(path, base string) error {
	base = filepath.Clean(base)
	rel, err := filepath.Rel(base, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("数据集路径必须位于 %s 下", base)
	}
	return nil
}

func (s *datasetService) removeOwnedDatasetArtifact(dataset *types.Dataset) error {
	cleanPath := filepath.Clean(dataset.FilePath)
	if err := s.ensurePathUnderDatasetBase(dataset, cleanPath); err != nil {
		return err
	}

	parentDir := filepath.Dir(cleanPath)
	if filepath.Base(parentDir) == dataset.ID {
		return os.RemoveAll(parentDir)
	}
	return os.RemoveAll(cleanPath)
}

func fillDatasetNames(datasets []*types.Dataset) {
	for _, dataset := range datasets {
		fillDatasetName(dataset)
	}
}

func fillDatasetName(dataset *types.Dataset) {
	if dataset == nil {
		return
	}
	if path := strings.TrimSpace(dataset.FilePath); path != "" {
		cleanPath := filepath.Clean(strings.ReplaceAll(path, "\\", "/"))
		name := filepath.Base(cleanPath)
		if name != "." && name != string(filepath.Separator) && name != "" {
			dataset.DatasetName = name
			return
		}
	}
	if strings.TrimSpace(dataset.DatasetName) != "" {
		return
	}
	dataset.DatasetName = dataset.Name
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
