package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestRunCachesAndStageLogsRemainInUserStorage(t *testing.T) {
	root := t.TempDir()
	e := NewStageExecutor(root, nil, nil, "data", nil, nil)
	p := &types.PipelineRun{UID: 380, ProjectID: "project", ID: "run", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{}}}
	project := &types.Project{UID: 380, ID: "project", TeacherModelConfig: types.ModelConfig{Endpoint: "http://teacher/v1", ModelName: "teacher"}}
	stage := &types.StageRun{StageType: types.StageTeacherConfig}
	if err := e.ExecuteStage(context.Background(), stage, p, project); err != nil {
		t.Fatal(err)
	}
	workspace := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	for _, dir := range []string{"data/seed", "data/intermediate", "data/exported", "models/checkpoints", "cache/huggingface/datasets", "tmp"} {
		if info, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(dir))); err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", dir, err)
		}
	}
	logPath := filepath.Join(workspace, "logs", "teacher_config", "events.log")
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), `"status":"started"`) || !strings.Contains(string(data), `"status":"succeeded"`) {
		t.Fatalf("missing durable lifecycle log: %s %v", data, err)
	}
	project.TeacherModelConfig.Endpoint = ""
	if err := e.ExecuteStage(context.Background(), stage, p, project); err == nil {
		t.Fatal("missing teacher accepted")
	}
	data, _ = os.ReadFile(logPath)
	if !strings.Contains(string(data), `"status":"failed"`) {
		t.Fatal("failure log was not retained")
	}
}

func TestContainerRequestPinsCachesToItsOwnRun(t *testing.T) {
	var request gcsclient.ContainerTaskRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"accepted":true,"container_name":"data"}`))
	}))
	defer server.Close()
	e := NewStageExecutor(t.TempDir(), nil, gcsclient.NewClient(server.URL, time.Second), "data", nil, nil)
	root := e.configGen.GetRunWorkspace(380, "project", "run")
	_, err := e.runContainerTask(context.Background(), &ContainerRequest{TaskUID: 380, ContainerName: "data", Image: "data", HostWorkDir: root, ConfigPath: filepath.Join(root, "configs", "data.json"), ResourceMode: "cpu"})
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, item := range strings.Split(request.Envs, ";") {
		key, value, _ := strings.Cut(item, "=")
		env[key] = value
	}
	for _, key := range []string{"HOME", "TMPDIR", "HF_HOME", "HF_DATASETS_CACHE", "TORCH_HOME", "TORCH_EXTENSIONS_DIR", "TRITON_CACHE_DIR", "MPLCONFIGDIR", "XDG_CACHE_HOME"} {
		rel, err := filepath.Rel(root, env[key])
		if env[key] == "" || err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			t.Fatalf("%s escaped run: %q", key, env[key])
		}
	}
	if env["HF_HUB_OFFLINE"] != "1" || request.ImagePullPolicy != "never" {
		t.Fatal("offline policy missing")
	}
	_, err = e.runContainerTask(context.Background(), &ContainerRequest{TaskUID: 381, ContainerName: "foreign", Image: "data", HostWorkDir: root, ConfigPath: filepath.Join(root, "configs", "data.json"), ResourceMode: "cpu"})
	if err == nil {
		t.Fatal("another user's workspace accepted")
	}
}

func TestStorageDirectoryRejectsEscapesBeforeCreatingChildren(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureStorageDir(root, "../outside"); err == nil {
		t.Fatal("path traversal accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "user-380")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := EnsureStorageDir(root, "user-380/private"); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "private")); !os.IsNotExist(err) {
		t.Fatal("created directory through foreign symlink")
	}
}
