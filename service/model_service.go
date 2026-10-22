package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/runtime"
)

type LocalModel struct {
	Ready             bool   `json:"ready"`
	ReadinessError    string `json:"readiness_error,omitempty"`
	ModelType         string `json:"model_type,omitempty"`
	SupportsVision    bool   `json:"supports_vision"`
	SuggestedTemplate string `json:"suggested_template,omitempty"`
	ID                string `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	Description       string `json:"description"`
	Size              int64  `json:"size"`
}

type StudentModel = LocalModel

type ModelService interface {
	ListStudentModels(ctx context.Context, uid int) ([]*StudentModel, error)
	GetStudentModel(ctx context.Context, uid int, modelID string) (*StudentModel, error)
	ValidateStudentModel(ctx context.Context, uid int, modelPath string) error
}

type modelService struct{ storage config.StorageConfig }

func NewModelService(storageCfg *config.StorageConfig) ModelService {
	storage := config.Default().Storage
	if storageCfg != nil && strings.TrimSpace(storageCfg.RootPath) != "" {
		storage = *storageCfg
	}
	return &modelService{storage: storage}
}

func (s *modelService) base(uid int) (string, error) {
	base, err := s.storage.UserStudentModels(uid)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(s.storage.RootPath, base)
	if err != nil {
		return "", err
	}
	return runtime.EnsureStorageDir(s.storage.RootPath, rel)
}

func (s *modelService) ListStudentModels(ctx context.Context, uid int) ([]*StudentModel, error) {
	base, err := s.base(uid)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	models := []*StudentModel{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		model, err := s.GetStudentModel(ctx, uid, entry.Name())
		if err == nil {
			models = append(models, model)
		}
	}
	return models, nil
}

func (s *modelService) GetStudentModel(ctx context.Context, uid int, modelID string) (*StudentModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if modelID == "" || modelID == "." || strings.Contains(modelID, "..") || strings.ContainsAny(modelID, "/\\") {
		return nil, fmt.Errorf("无效的学生模型 ID")
	}
	base, err := s.base(uid)
	if err != nil {
		return nil, err
	}
	path, err := runtime.SafeArtifactPath(base, modelID)
	if err != nil {
		return nil, fmt.Errorf("学生模型不在当前用户模型目录内")
	}
	size, err := modelTreeSize(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(filepath.Join(path, "config.json")); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("学生模型缺少 config.json")
	}
	m := &StudentModel{ID: modelID, Name: modelID, Path: path, Size: size}
	fillModelReadiness(m)
	return m, nil
}

func (s *modelService) ValidateStudentModel(ctx context.Context, uid int, modelPath string) error {
	base, err := s.base(uid)
	if err != nil {
		return err
	}
	if !isSubPath(base, modelPath) || filepath.Clean(modelPath) == base {
		return fmt.Errorf("学生模型必须位于当前用户目录 %s 下", base)
	}
	rel, err := filepath.Rel(base, modelPath)
	if err != nil || filepath.Base(rel) != rel {
		return fmt.Errorf("学生模型必须是当前用户模型目录的直接子目录")
	}
	m, err := s.GetStudentModel(ctx, uid, rel)
	if err != nil {
		return err
	}
	if !m.Ready {
		return fmt.Errorf("%s", m.ReadinessError)
	}
	return nil
}

func modelTreeSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("学生模型目录不允许符号链接，请导入完整文件")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("学生模型包含非常规文件")
		}
		size += info.Size()
		return nil
	})
	return size, err
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
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)))
}
