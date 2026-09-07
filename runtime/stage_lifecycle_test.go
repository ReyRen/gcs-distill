package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
)

type lifecycleStageRepo struct {
	mysqlrepo.StageRepository
	saved chan types.StageRun
}

func (r *lifecycleStageRepo) Update(_ context.Context, stage *types.StageRun) error {
	r.saved <- *stage
	return nil
}

type lifecyclePipelineRepo struct {
	mysqlrepo.PipelineRepository
	status types.PipelineStatus
}

func (r *lifecyclePipelineRepo) GetByID(_ context.Context, id string) (*types.PipelineRun, error) {
	return &types.PipelineRun{ID: id, Status: r.status}, nil
}

func TestFailedTeacherTaskKeepsPublishedLogTarget(t *testing.T) {
	saved := make(chan types.StageRun, 1)
	var published atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"accepted":true,"container_name":"teacher-task"}`))
		case http.MethodGet:
			select {
			case stage := <-saved:
				published.Store(stage.ContainerID == "teacher-task" && stage.LogPath != "" && stage.ConfigPath != "")
			default:
			}
			_, _ = w.Write([]byte(`{"task_states":12}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	executor := NewStageExecutor(t.TempDir(), nil, gcsclient.NewClient(server.URL, time.Second), "runtime",
		&lifecycleStageRepo{saved: saved}, &lifecyclePipelineRepo{status: types.StatusRunning})
	stage := &types.StageRun{ID: "stage", StageType: types.StageTeacherInfer}
	project := &types.Project{ID: "project", UID: 1, TeacherModelConfig: types.ModelConfig{ProviderType: types.ProviderLocal, ModelName: "model", ModelPath: "/models/model"}}
	err := executor.executeTeacherInfer(context.Background(), stage, project, &types.PipelineRun{ID: "pipeline", UID: 1})
	if err == nil || !strings.Contains(err.Error(), "state=12") {
		t.Fatalf("expected worker failure, got %v", err)
	}
	if !published.Load() || stage.ContainerID != "teacher-task" {
		t.Fatalf("log target was not persisted before completion: %+v", stage)
	}
}

func TestCanceledPipelineDeletesItsGCSTask(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/tasks/teacher-task" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		deleted.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	executor := NewStageExecutor(t.TempDir(), nil, gcsclient.NewClient(server.URL, time.Second), "runtime", nil, &lifecyclePipelineRepo{status: types.StatusCanceled})
	err := executor.waitForContainerTask(context.Background(), "teacher-task", "pipeline")
	if !errors.Is(err, context.Canceled) || !deleted.Load() {
		t.Fatalf("canceled pipeline left its task running: deleted=%v err=%v", deleted.Load(), err)
	}
}
