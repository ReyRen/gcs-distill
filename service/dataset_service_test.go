package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
)

const testUID = 380

func TestDatasetBasePathDefaultsToUserModelDistillDatasets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage-root-jfs")
	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}

	want := filepath.Join(root, "user-380", "train-center", "model-distill", "datasets")
	got, err := svc.datasetBasePath(testUID)
	if err != nil {
		t.Fatalf("datasetBasePath() error = %v", err)
	}
	if got != want {
		t.Fatalf("datasetBasePath() = %q, want %q", got, want)
	}
}

func TestListDatasetCandidatesScansControlledCandidatePath(t *testing.T) {
	root := t.TempDir()
	candidatesDir := filepath.Join(root, "user-380", "train-center", "model-distill", "datasets", "candidates")
	uploadsDir := filepath.Join(root, "user-380", "train-center", "model-distill", "datasets", "uploaded")
	mustWriteFile(t, filepath.Join(candidatesDir, "alpha", "train.jsonl"), []byte("{\"a\":1}\n{\"a\":2}\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, "beta.txt"), []byte("one\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, "skip.csv"), []byte("ignored\n"))
	mustWriteFile(t, filepath.Join(candidatesDir, ".hidden.jsonl"), []byte("ignored\n"))
	mustWriteFile(t, filepath.Join(uploadsDir, "uploaded.jsonl"), []byte("should not be listed\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}
	items, err := svc.ListDatasetCandidates(context.Background(), testUID)
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
	root := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "outside.jsonl")
	mustWriteFile(t, outsideFile, []byte("{}\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}
	if err := svc.ensureDatasetFilePath(testUID, outsideFile); err == nil {
		t.Fatal("ensureDatasetFilePath() error = nil, want outside-base error")
	}
}

func TestEnsureDatasetFilePathOnlyAcceptsCandidatePath(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "user-380", "train-center", "model-distill", "datasets")
	candidateFile := filepath.Join(baseDir, "candidates", "seed.jsonl")
	uploadedFile := filepath.Join(baseDir, "uploaded", "seed.jsonl")
	mustWriteFile(t, candidateFile, []byte("{}\n"))
	mustWriteFile(t, uploadedFile, []byte("{}\n"))

	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}
	if err := svc.ensureDatasetFilePath(testUID, candidateFile); err != nil {
		t.Fatalf("ensureDatasetFilePath(candidate) error = %v", err)
	}
	if err := svc.ensureDatasetFilePath(testUID, uploadedFile); err == nil {
		t.Fatal("ensureDatasetFilePath(uploaded) error = nil, want candidate-path error")
	}
}

func TestGetDatasetPathUsesUserUploadsPath(t *testing.T) {
	root := t.TempDir()
	svc := &datasetService{storageCfg: &config.StorageConfig{RootPath: root}}

	want := filepath.Join(root, "user-380", "train-center", "model-distill", "datasets", "uploaded", "dataset-1")
	got, err := svc.GetDatasetPath(testUID, "dataset-1")
	if err != nil {
		t.Fatalf("GetDatasetPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("GetDatasetPath() = %q, want %q", got, want)
	}
}

func TestCreateDatasetRejectsJSONUploadSourceType(t *testing.T) {
	svc := &datasetService{}

	err := svc.CreateDataset(context.Background(), &types.Dataset{
		UID:        testUID,
		Name:       "upload.jsonl",
		SourceType: "upload",
	})
	if err == nil {
		t.Fatal("CreateDataset() error = nil, want multipart upload guidance")
	}
}

func TestUpdateDatasetPreservesStoredSource(t *testing.T) {
	repo := &fakeDatasetRepo{dataset: &types.Dataset{
		ID:          "dataset-1",
		UID:         testUID,
		Name:        "old name",
		Description: "old description",
		SourceType:  "upload",
		FilePath:    "/storage-root-jfs/user-380/train-center/model-distill/datasets/uploaded/dataset-1/train.jsonl",
		RecordCount: 7,
	}}
	svc := &datasetService{datasetRepo: repo}

	err := svc.UpdateDataset(context.Background(), &types.Dataset{
		ID:          "dataset-1",
		UID:         testUID,
		Name:        "new name",
		Description: "new description",
		SourceType:  "import",
		FilePath:    "/tmp/should-not-be-used.jsonl",
		RecordCount: 999,
	})
	if err != nil {
		t.Fatalf("UpdateDataset() error = %v", err)
	}

	if repo.dataset.Name != "new name" {
		t.Fatalf("name = %q, want new name", repo.dataset.Name)
	}
	if repo.dataset.Description != "new description" {
		t.Fatalf("description = %q, want new description", repo.dataset.Description)
	}
	if repo.dataset.SourceType != "upload" {
		t.Fatalf("source_type = %q, want upload", repo.dataset.SourceType)
	}
	if repo.dataset.FilePath != "/storage-root-jfs/user-380/train-center/model-distill/datasets/uploaded/dataset-1/train.jsonl" {
		t.Fatalf("file_path = %q, want stored path", repo.dataset.FilePath)
	}
	if repo.dataset.RecordCount != 7 {
		t.Fatalf("record_count = %d, want 7", repo.dataset.RecordCount)
	}
}

type fakeDatasetRepo struct {
	dataset *types.Dataset
}

func (r *fakeDatasetRepo) Create(_ context.Context, dataset *types.Dataset) error {
	copy := *dataset
	r.dataset = &copy
	return nil
}

func (r *fakeDatasetRepo) GetByID(_ context.Context, id string) (*types.Dataset, error) {
	if r.dataset == nil || r.dataset.ID != id {
		return nil, fmt.Errorf("dataset not found: %s", id)
	}
	copy := *r.dataset
	return &copy, nil
}

func (r *fakeDatasetRepo) List(_ context.Context, _, _, _ int) ([]*types.Dataset, error) {
	return nil, nil
}

func (r *fakeDatasetRepo) Update(_ context.Context, dataset *types.Dataset) error {
	copy := *dataset
	r.dataset = &copy
	return nil
}

func (r *fakeDatasetRepo) Delete(_ context.Context, id string) error {
	if r.dataset != nil && r.dataset.ID == id {
		r.dataset = nil
	}
	return nil
}

func (r *fakeDatasetRepo) Count(_ context.Context, _ int) (int, error) {
	return 0, nil
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
