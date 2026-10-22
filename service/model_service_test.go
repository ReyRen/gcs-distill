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

func TestModelServiceListsStudentsFromUserBasePath(t *testing.T) {
	ctx := context.Background()
	storage := config.StorageConfig{RootPath: t.TempDir()}
	basePath, _ := storage.UserStudentModels(380)
	validPath := filepath.Join(basePath, "Qwen2.5-0.5B-Instruct")
	invalidPath := filepath.Join(basePath, "missing-config")

	mustMkdirAll(t, validPath)
	mustWriteModelFile(t, filepath.Join(validPath, "config.json"), []byte(`{"model_type":"qwen2"}`))
	mustMkdirAll(t, invalidPath)

	svc := NewModelService(&storage)

	studentModels, err := svc.ListStudentModels(ctx, 380)
	if err != nil {
		t.Fatalf("ListStudentModels() error = %v", err)
	}
	assertSingleModel(t, studentModels, "Qwen2.5-0.5B-Instruct", validPath)

	studentModel, err := svc.GetStudentModel(ctx, 380, "Qwen2.5-0.5B-Instruct")
	if err != nil {
		t.Fatalf("GetStudentModel() error = %v", err)
	}
	if studentModel.Path != validPath {
		t.Fatalf("GetStudentModel() path = %q, want %q", studentModel.Path, validPath)
	}

	if _, err := svc.GetStudentModel(ctx, 380, "../Qwen2.5-0.5B-Instruct"); err == nil {
		t.Fatalf("GetStudentModel() expected path traversal error")
	}
	if other, err := svc.ListStudentModels(ctx, 381); err != nil || len(other) != 0 {
		t.Fatalf("another user saw private models: %v %v", other, err)
	}
	if _, err := svc.GetStudentModel(ctx, 381, "Qwen2.5-0.5B-Instruct"); err == nil {
		t.Fatal("another user resolved a private model")
	}
	if _, err := svc.ListStudentModels(ctx, 0); err == nil {
		t.Fatal("missing UID accepted")
	}
}

func TestModelServiceValidateStudentModelRejectsSiblingPrefixPath(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storage := config.StorageConfig{RootPath: root}
	basePath, _ := storage.UserStudentModels(380)
	validPath := filepath.Join(basePath, "valid")
	siblingPath := filepath.Join(root, "models-extra", "invalid")

	mustMkdirAll(t, validPath)
	mustWriteModelFile(t, filepath.Join(validPath, "config.json"), []byte(`{}`))
	mustWriteModelFile(t, filepath.Join(validPath, "tokenizer_config.json"), []byte(`{}`))
	mustWriteModelFile(t, filepath.Join(validPath, "tokenizer.json"), []byte(`{}`))
	mustWriteModelFile(t, filepath.Join(validPath, "model.safetensors"), []byte("fixture"))
	mustMkdirAll(t, siblingPath)
	mustWriteModelFile(t, filepath.Join(siblingPath, "config.json"), []byte(`{}`))

	svc := NewModelService(&storage)

	if err := svc.ValidateStudentModel(ctx, 380, validPath); err != nil {
		t.Fatalf("ValidateStudentModel(valid) error = %v", err)
	}
	if err := svc.ValidateStudentModel(ctx, 380, siblingPath); err == nil {
		t.Fatalf("ValidateStudentModel(sibling) expected boundary error")
	}
	if err := svc.ValidateStudentModel(ctx, 381, validPath); err == nil {
		t.Fatal("another user's model path accepted")
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
