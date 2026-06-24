package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/logger"
)

func init() {
	_ = logger.Initialize(&config.LoggingConfig{
		Level:  "error",
		Output: "stdout",
	})
}

func TestModelServiceListsTeacherAndStudentFromSharedBasePath(t *testing.T) {
	ctx := context.Background()
	basePath := t.TempDir()
	validPath := filepath.Join(basePath, "Qwen2.5-0.5B-Instruct")
	invalidPath := filepath.Join(basePath, "missing-config")

	mustMkdirAll(t, validPath)
	mustWriteModelFile(t, filepath.Join(validPath, "config.json"), []byte(`{"model_type":"qwen2"}`))
	mustMkdirAll(t, invalidPath)

	svc := NewModelService(&config.StorageConfig{ModelsBasePath: basePath})

	teacherModels, err := svc.ListTeacherModels(ctx)
	if err != nil {
		t.Fatalf("ListTeacherModels() error = %v", err)
	}
	assertSingleModel(t, teacherModels, "Qwen2.5-0.5B-Instruct", validPath)

	studentModels, err := svc.ListStudentModels(ctx)
	if err != nil {
		t.Fatalf("ListStudentModels() error = %v", err)
	}
	assertSingleModel(t, studentModels, "Qwen2.5-0.5B-Instruct", validPath)

	teacherModel, err := svc.GetTeacherModel(ctx, "Qwen2.5-0.5B-Instruct")
	if err != nil {
		t.Fatalf("GetTeacherModel() error = %v", err)
	}
	if teacherModel.Path != validPath {
		t.Fatalf("GetTeacherModel() path = %q, want %q", teacherModel.Path, validPath)
	}

	if _, err := svc.GetTeacherModel(ctx, "../Qwen2.5-0.5B-Instruct"); err == nil {
		t.Fatalf("GetTeacherModel() expected path traversal error")
	}
}

func TestModelServiceValidateStudentModelRejectsSiblingPrefixPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	basePath := filepath.Join(root, "models")
	validPath := filepath.Join(basePath, "valid")
	siblingPath := filepath.Join(root, "models-extra", "invalid")

	mustMkdirAll(t, validPath)
	mustWriteModelFile(t, filepath.Join(validPath, "config.json"), []byte(`{}`))
	mustMkdirAll(t, siblingPath)
	mustWriteModelFile(t, filepath.Join(siblingPath, "config.json"), []byte(`{}`))

	svc := NewModelService(&config.StorageConfig{ModelsBasePath: basePath})

	if err := svc.ValidateStudentModel(ctx, validPath); err != nil {
		t.Fatalf("ValidateStudentModel(valid) error = %v", err)
	}
	if err := svc.ValidateStudentModel(ctx, siblingPath); err == nil {
		t.Fatalf("ValidateStudentModel(sibling) expected boundary error")
	}
}

func assertSingleModel(t *testing.T, models []*LocalModel, wantID string, wantPath string) {
	t.Helper()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].ID != wantID {
		t.Fatalf("model ID = %q, want %q", models[0].ID, wantID)
	}
	if models[0].Path != wantPath {
		t.Fatalf("model path = %q, want %q", models[0].Path, wantPath)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteModelFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
