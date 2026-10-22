package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

type artifactPipeline struct {
	service.PipelineService
	pipeline *types.PipelineRun
	stage    *types.StageRun
}

func (p artifactPipeline) GetPipeline(context.Context, string) (*types.PipelineRun, error) {
	return p.pipeline, nil
}
func (p artifactPipeline) GetStage(context.Context, string) (*types.StageRun, error) {
	return p.stage, nil
}

func TestModelZipRequiresWeightsAndExcludesSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	p := &types.PipelineRun{UID: 369, ID: "run", ProjectID: "project"}
	root := runtime.NewConfigGenerator(storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	h := &WorkflowHandler{storage: storage, pipelines: artifactPipeline{pipeline: p}}
	r := gin.New()
	r.GET("/pipelines/:id/model/download", h.DownloadModel)
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/pipelines/run/model/download?uid=369", nil))
		return w
	}
	if request().Code != 404 {
		t.Fatal("missing weights marked downloadable")
	}
	for name, data := range map[string]string{"models/checkpoints/adapter_model.safetensors": "weights", "models/checkpoints/adapter_config.json": "{}", "models/checkpoints/private.env": "secret", "configs/teacher.json": "secret"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(data), 0600)
	}
	w := request()
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(z.File) != 2 {
		t.Fatal(z, err)
	}
	for _, f := range z.File {
		if f.Name != "adapter_model.safetensors" && f.Name != "adapter_config.json" {
			t.Fatal("unexpected ZIP entry", f.Name)
		}
	}
}

func TestArchivedLogsSurviveContainerRemoval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	p := &types.PipelineRun{UID: 369, ID: "run", ProjectID: "project"}
	root := runtime.NewConfigGenerator(storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	logDir := filepath.Join(root, "logs", "student_train")
	os.MkdirAll(logDir, 0755)
	os.WriteFile(filepath.Join(logDir, "archived.log"), []byte("first\nlast\n"), 0600)
	stage := &types.StageRun{ID: "stage", PipelineRunID: p.ID, ContainerID: "removed", LogPath: logDir}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("queried removed container"); w.WriteHeader(404) }))
	defer upstream.Close()
	h := NewPipelineHandler(artifactPipeline{pipeline: p, stage: stage}, gcsclient.NewClient(upstream.URL, time.Second), storage)
	r := gin.New()
	r.GET("/pipelines/:id/stages/:stage_id/logs", h.GetStageLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/pipelines/run/stages/stage/logs?tail=1", nil))
	if w.Code != 200 || w.Body.String() != "last" {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestControlStageLogsAreAvailableWithoutContainer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	p := &types.PipelineRun{UID: 369, ID: "run", ProjectID: "project"}
	root := runtime.NewConfigGenerator(storage.RootPath).GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	logDir := filepath.Join(root, "logs", "teacher_config")
	os.MkdirAll(logDir, 0750)
	os.WriteFile(filepath.Join(logDir, "events.log"), []byte("started\nsucceeded\n"), 0600)
	stage := &types.StageRun{ID: "stage", PipelineRunID: p.ID, LogPath: logDir}
	h := NewPipelineHandler(artifactPipeline{pipeline: p, stage: stage}, gcsclient.NewClient("http://unused", time.Second), storage)
	r := gin.New()
	r.GET("/pipelines/:id/stages/:stage_id/logs/download", h.DownloadStageLogs)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/pipelines/run/stages/stage/logs/download", nil))
	if w.Code != 200 || w.Body.String() != "started\nsucceeded\n" {
		t.Fatal(w.Code, w.Body.String())
	}
}
