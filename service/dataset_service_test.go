package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestDatasetBasePathDefaultsToUserModelDistillDatasets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage-root-jfs", "user-001")
	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}

	want := filepath.Join(root, "train-center", "model-distill", "datasets")
	if got := svc.datasetBasePath(); got != want {
		t.Fatalf("datasetBasePath() = %q, want %q", got, want)
	}
}

func TestListDatasetCandidatesScansControlledCandidatePath(t *testing.T) {
	baseDir := t.TempDir()
	candidatesDir := filepath.Join(baseDir, "candidates")
	uploadsDir := filepath.Join(baseDir, "uploaded")
	mustWriteFile(t, filepath.Join(candidatesDir, "alpha", "train.jsonl"), []byte("{\"a\":1}\n{\"a\":2}\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, "beta.txt"), []byte("one\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, "skip.csv"), []byte("ignored\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, ".hidden.jsonl"), []byte("ignored\n"))
	mustWriteFile(t, filepath.Join(uploadsDir, "uploaded.jsonl"), []byte("should not be listed\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{DatasetsBasePath: baseDir}}
	items, err := svc.ListDatasetCandidates(context.Background())
	if err != nil {
		t.Fatalf("ListDatasetCandidates() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("candidate count = %d, want 2: %#v", len(items), items)
	}

	if items[0].Name != filepath.Join("alpha", "train.jsonl") {
		t.Fatalf("first candidate name = %q", items[0].Name)
	}
	if items[0].RecordCount != 2 {
		t.Fatalf("first candidate record_count = %d, want 2", items[0].RecordCount)
	}
	if items[1].Name != "beta.txt" {
		t.Fatalf("second candidate name = %q", items[1].Name)
	}
}

func TestEnsureDatasetFilePathRejectsOutsideBase(t *testing.T) {
	baseDir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "outside.jsonl")
	mustWriteFile(t, outsideFile, []byte("{}\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{DatasetsBasePath: baseDir}}
	if err := svc.ensureDatasetFilePath(outsideFile); err == nil {
		t.Fatal("ensureDatasetFilePath() error = nil, want outside-base error")
	}
}

func TestEnsureDatasetFilePathOnlyAcceptsCandidatePath(t *testing.T) {
	baseDir := t.TempDir()
	candidateFile := filepath.Join(baseDir, "candidates", "seed.jsonl")
	uploadedFile := filepath.Join(baseDir, "uploaded", "seed.jsonl")
	mustWriteFile(t, candidateFile, []byte("{}\n"))
	mustWriteFile(t, uploadedFile, []byte("{}\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{DatasetsBasePath: baseDir}}
	if err := svc.ensureDatasetFilePath(candidateFile); err != nil {
		t.Fatalf("ensureDatasetFilePath(candidate) error = %v", err)
	}
	if err := svc.ensureDatasetFilePath(uploadedFile); err == nil {
		t.Fatal("ensureDatasetFilePath(uploaded) error = nil, want candidate-path error")
	}
}

func TestGetDatasetPathUsesUploadsPath(t *testing.T) {
	baseDir := t.TempDir()
	svc := &datasetService{storageCfg: &config.StorageConfig{DatasetsBasePath: baseDir}}

	want := filepath.Join(baseDir, "uploaded", "dataset-1")
	if got := svc.GetDatasetPath("project-1", "dataset-1"); got != want {
		t.Fatalf("GetDatasetPath() = %q, want %q", got, want)
	}
}

func TestCreateDatasetRejectsJSONUploadSourceType(t *testing.T) {
	svc := &datasetService{}

	err := svc.CreateDataset(context.Background(), &types.Dataset{
		ProjectID:  "project-1",
		Name:       "upload.jsonl",
		SourceType: "upload",
	})
	if err == nil {
		t.Fatal("CreateDataset() error = nil, want multipart upload guidance")
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
