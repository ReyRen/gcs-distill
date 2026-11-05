package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

type splitDatasetService struct {
	service.DatasetService
	d *types.Dataset
}

func (s splitDatasetService) GetDataset(_ context.Context, uid int, id string) (*types.Dataset, error) {
	if s.d.UID != uid || s.d.ID != id {
		return nil, fmt.Errorf("not found")
	}
	return s.d, nil
}

func TestDatasetSplitAPILifecycleOwnershipAndTrainingHandoff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	base, _ := storage.UserDatasetGenerated(369)
	dir := filepath.Join(base, "asset")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "dataset.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for i := 0; i < 30; i++ {
		if err := enc.Encode(map[string]interface{}{"messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("客服问题%d", i)}, {"role": "assistant", "content": "回答"}}}); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	h := &WorkflowHandler{storage: storage, datasets: splitDatasetService{d: &types.Dataset{ID: "asset", UID: 369, SourceType: "generated", FilePath: path, Lineage: map[string]string{"job_type": "instruct_distill"}}}}
	r := gin.New()
	r.POST("/datasets/:id/splits/preview", h.PreviewDatasetSplit)
	r.POST("/datasets/:id/splits", h.CreateDatasetSplit)
	r.GET("/datasets/:id/splits", h.ListDatasetSplits)
	r.GET("/datasets/:id/splits/:split_id", h.GetDatasetSplit)
	r.GET("/datasets/:id/splits/:split_id/partitions/:partition/preview", h.PreviewSplitPartition)
	r.GET("/datasets/:id/splits/:split_id/partitions/:partition/download", h.DownloadSplitPartition)
	r.GET("/datasets/:id/training-config", h.DatasetTrainingConfig)
	req := func(method, url, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		q := httptest.NewRequest(method, url, bytes.NewBufferString(body))
		q.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, q)
		return w
	}
	body := `{"enabled":true,"train_ratio":0.8,"validation_ratio":0.1,"test_ratio":0.1,"seed":42}`
	w := req("POST", "/datasets/asset/splits/preview?uid=369", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "splits")); !os.IsNotExist(err) {
		t.Fatal("preview persisted files")
	}
	var response struct {
		Data struct {
			ID string `json:"id"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	id := response.Data.ID
	w = req("POST", "/datasets/asset/splits?uid=369", body)
	if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, endpoint := range []string{"/splits", "/splits/" + id, "/splits/" + id + "/partitions/train/preview", "/splits/" + id + "/partitions/validation/download", "/splits/" + id + "/partitions/test/download"} {
		w = req("GET", "/datasets/asset"+endpoint+"?uid=369", "")
		if w.Code != 200 {
			t.Fatal(endpoint, w.Code, w.Body.String())
		}
		w = req("GET", "/datasets/asset"+endpoint+"?uid=338", "")
		if w.Code != 404 {
			t.Fatal("cross-user split access", w.Code)
		}
	}
	w = req("GET", "/datasets/asset/training-config?uid=369&split_id="+id, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"eval_dataset":"gcs_validation"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	var training struct {
		Data struct {
			Info map[string]interface{} `json:"dataset_info"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &training); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(training.Data.Info)
	if strings.Contains(string(b), "test.jsonl") || !strings.Contains(string(b), "train.jsonl") {
		t.Fatal("held-out data used for training", string(b))
	}
	if w = req("GET", "/datasets/asset/training-config?uid=369&split_id=bad", ""); w.Code != 422 {
		t.Fatal("bad split silently ignored", w.Code)
	}
	if w = req("POST", "/datasets/asset/splits?uid=369", `{"enabled":true,"train_ratio":1}`); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = req("POST", "/datasets/asset/splits?uid=369", `{"enabled":true,"bogus":123}`); w.Code != 400 {
		t.Fatal("unknown config accepted", w.Code)
	}
}
