package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

func TestArtifactCountsAndSampleQualityHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	p := &types.PipelineRun{UID: 338, ID: "run", ProjectID: "project"}
	root := runtime.NewConfigGenerator(storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	for name, data := range map[string]string{
		"data/exported/dataset.jsonl":               `{"messages":[{"role":"user","content":"Q"},{"role":"assistant","content":"A"}]}`,
		"data/exported/manifest.json":               `{"record_count":"1"}`,
		"data/intermediate/04_instruct_eval.jsonl":  `{"instruction":"Q","response":"A","correctness":false,"helpfulness":0}`,
		"data/intermediate/05_quality_filter.jsonl": "",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := &WorkflowHandler{storage: storage, pipelines: artifactPipeline{pipeline: p}}
	r := gin.New()
	r.GET("/pipelines/:id/artifacts", h.Artifacts)
	r.GET("/pipelines/:id/artifacts/preview", h.PreviewArtifact)
	r.GET("/pipelines/:id/quality", h.Quality)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/pipelines/run/"+path, nil))
		return w
	}
	w := get("artifacts?uid=338")
	var list struct {
		Data struct {
			Items []runtime.Artifact `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list.Data.Items) != 4 {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	for _, item := range list.Data.Items {
		if item.Name == "manifest.json" && (item.CountStatus != "not_applicable" || item.RecordCount != nil) {
			t.Fatal(item)
		}
		if item.Name == "dataset.jsonl" && (item.RecordCount == nil || *item.RecordCount != 1) {
			t.Fatal(item)
		}
	}
	for _, path := range []string{"data/exported/dataset.jsonl", "data/intermediate/05_quality_filter.jsonl"} {
		w = get("artifacts/preview?uid=338&path=" + path)
		var result struct {
			Data struct {
				Quality runtime.PreviewQuality `json:"quality"`
				Total   int                    `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		if path == "data/exported/dataset.jsonl" && (result.Data.Quality.Scored != 1 || result.Data.Quality.Items[0].Metrics["correctness"] != false) {
			t.Fatal(w.Body.String())
		}
		if path == "data/intermediate/05_quality_filter.jsonl" && result.Data.Total != 0 {
			t.Fatal(w.Body.String())
		}
	}
	w = get("quality?uid=338")
	var report struct {
		Data runtime.QualityStatistics `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || w.Code != 200 || report.Data.ExportedDataset.Scored != 1 {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	for _, path := range []string{"artifacts?uid=369", "quality?uid=369", "artifacts/preview?uid=369&path=data/exported/dataset.jsonl"} {
		if w := get(path); w.Code != 404 {
			t.Fatal("foreign task disclosed", w.Code)
		}
	}
}
