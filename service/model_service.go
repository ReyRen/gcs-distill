package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"go.uber.org/zap"
)

// LocalModel 本地模型信息
type LocalModel struct {
	ID          string `json:"id"`          // 模型ID（目录名）
	Name        string `json:"name"`        // 模型名称
	Path        string `json:"path"`        // 模型路径
	Description string `json:"description"` // 模型描述
	Size        int64  `json:"size"`        // 模型大小（字节）
}

type TeacherModel = LocalModel
type StudentModel = LocalModel

// ModelService 模型服务接口
type ModelService interface {
	// ListTeacherModels 列出所有可用的本地教师模型
	ListTeacherModels(ctx context.Context) ([]*TeacherModel, error)
	// GetTeacherModel 获取指定本地教师模型信息
	GetTeacherModel(ctx context.Context, modelID string) (*TeacherModel, error)
	// ListStudentModels 列出所有可用的学生模型
	ListStudentModels(ctx context.Context) ([]*StudentModel, error)
	// GetStudentModel 获取指定学生模型信息
	GetStudentModel(ctx context.Context, modelID string) (*StudentModel, error)
	// ValidateLocalModel 验证本地模型是否存在且可用
	ValidateLocalModel(ctx context.Context, modelPath string) error
	// ValidateStudentModel 验证学生模型是否存在且可用
	ValidateStudentModel(ctx context.Context, modelPath string) error
}

// modelService 模型服务实现
type modelService struct {
	modelsBasePath string
}

// NewModelService 创建模型服务
func NewModelService(storageCfg *config.StorageConfig) ModelService {
	modelsBasePath := ""
	if storageCfg != nil {
		modelsBasePath = strings.TrimSpace(storageCfg.ModelsBasePath)
	}
	if modelsBasePath == "" {
		// 默认使用 basePath/models
		root := "/storage-root-jfs"
		if storageCfg != nil && strings.TrimSpace(storageCfg.RootPath) != "" {
			root = strings.TrimRight(strings.TrimSpace(storageCfg.RootPath), "/")
		}
		modelsBasePath = filepath.Join(root, "train-base-models")
	}
	return &modelService{
		modelsBasePath: modelsBasePath,
	}
}

// ListTeacherModels 列出所有可用的本地教师模型
func (s *modelService) ListTeacherModels(ctx context.Context) ([]*TeacherModel, error) {
	_ = ctx
	return s.listLocalModels("teacher")
}

// GetTeacherModel 获取指定本地教师模型信息
func (s *modelService) GetTeacherModel(ctx context.Context, modelID string) (*TeacherModel, error) {
	_ = ctx
	return s.getLocalModel(modelID)
}

// ListStudentModels 列出所有可用的学生模型
func (s *modelService) ListStudentModels(ctx context.Context) ([]*StudentModel, error) {
	_ = ctx
	return s.listLocalModels("student")
}

// GetStudentModel 获取指定学生模型信息
func (s *modelService) GetStudentModel(ctx context.Context, modelID string) (*StudentModel, error) {
	_ = ctx
	return s.getLocalModel(modelID)
}

func (s *modelService) listLocalModels(role string) ([]*LocalModel, error) {
	// 检查模型目录是否存在
	if _, err := os.Stat(s.modelsBasePath); os.IsNotExist(err) {
		logger.Warn("本地模型目录不存在，返回空列表",
			zap.String("role", role),
			zap.String("path", s.modelsBasePath))
		return []*LocalModel{}, nil
	}

	// 读取目录
	entries, err := os.ReadDir(s.modelsBasePath)
	if err != nil {
		logger.Error("读取模型目录失败", zap.String("path", s.modelsBasePath), zap.Error(err))
		return nil, fmt.Errorf("读取模型目录失败: %w", err)
	}

	models := make([]*LocalModel, 0)
	for _, entry := range entries {
		// 只处理目录
		if !entry.IsDir() {
			continue
		}

		modelID := entry.Name()
		modelPath := filepath.Join(s.modelsBasePath, modelID)

		// 验证是否是有效的模型目录（至少包含 config.json）
		configPath := filepath.Join(modelPath, "config.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			logger.Debug("跳过无效模型目录（缺少config.json）",
				zap.String("model_id", modelID),
				zap.String("path", modelPath))
			continue
		}

		// 计算模型大小
		size := calculateDirSize(modelPath)

		// 尝试读取模型描述（可选）
		description := ""
		// TODO: 可以从 config.json 或 README.md 读取描述信息

		models = append(models, &LocalModel{
			ID:          modelID,
			Name:        modelID, // 默认使用目录名作为名称
			Path:        modelPath,
			Description: description,
			Size:        size,
		})
	}

	logger.Info("获取本地模型列表成功",
		zap.String("role", role),
		zap.Int("count", len(models)),
		zap.String("base_path", s.modelsBasePath))

	return models, nil
}

func (s *modelService) getLocalModel(modelID string) (*LocalModel, error) {
	// 安全检查：防止路径遍历攻击
	if strings.Contains(modelID, "..") || strings.Contains(modelID, "/") || strings.Contains(modelID, "\\") {
		return nil, fmt.Errorf("无效的模型ID: %s", modelID)
	}

	modelPath := filepath.Join(s.modelsBasePath, modelID)

	// 检查模型目录是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}

	// 验证是否是有效的模型目录
	configPath := filepath.Join(modelPath, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("无效的模型目录（缺少config.json）: %s", modelID)
	}

	// 计算模型大小
	size := calculateDirSize(modelPath)

	return &LocalModel{
		ID:          modelID,
		Name:        modelID,
		Path:        modelPath,
		Description: "",
		Size:        size,
	}, nil
}

// ValidateLocalModel 验证本地模型是否存在且可用
func (s *modelService) ValidateLocalModel(ctx context.Context, modelPath string) error {
	_ = ctx
	// 检查路径是否在允许的目录下
	if !isSubPath(s.modelsBasePath, modelPath) {
		return fmt.Errorf("模型路径必须在 %s 目录下", s.modelsBasePath)
	}

	// 检查模型目录是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("模型路径不存在: %s", modelPath)
	}

	// 验证是否包含必要的模型文件
	configPath := filepath.Join(modelPath, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("模型目录缺少 config.json: %s", modelPath)
	}

	return nil
}

// ValidateStudentModel 验证学生模型是否存在且可用
func (s *modelService) ValidateStudentModel(ctx context.Context, modelPath string) error {
	return s.ValidateLocalModel(ctx, modelPath)
}

func isSubPath(basePath, targetPath string) bool {
	baseAbs, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// calculateDirSize 计算目录大小
func calculateDirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		logger.Warn("计算目录大小失败", zap.String("path", path), zap.Error(err))
		return 0
	}
	return size
}
