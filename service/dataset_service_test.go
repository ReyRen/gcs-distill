package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
)

func TestDatasetBasePathDefaultsToInferCenterModelDistillDatasets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage-root-jfs")
	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}

	want := filepath.Join(root, "infer-center", "model-distill", "datasets")
	if got := svc.datasetBasePath(); got != want {
		t.Fatalf("datasetBasePath() = %q, want %q", got, want)
	}
}

func TestListDatasetCandidatesScansControlledDatasetBase(t *testing.T) {
	baseDir := t.TempDir()
	mustWriteFile(t, filepath.Join(baseDir, "alpha", "train.jsonl"), []byte("{\"a\":1}\n{\"a\":2}\n"))
	mustWriteFile(t, filepath.Join(baseDir, "beta.txt"), []byte("one\n"))
	mustWriteFile(t, filepath.Join(baseDir, "skip.csv"), []byte("ignored\n"))
	mustWriteFile(t, filepath.Join(baseDir, ".hidden.jsonl"), []byte("ignored\n"))

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

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
