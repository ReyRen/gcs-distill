package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
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
	stage := &types.StageRun{ID: "stage", StageType: types.StageDistillData}
	project := &types.Project{ID: "project", UID: 1, TeacherModelConfig: types.ModelConfig{ProviderType: types.ProviderAPI, ModelName: "model", Endpoint: "http://teacher/v1"}}
	err := executor.executeDistill2(context.Background(), stage, project, &types.PipelineRun{ID: "pipeline", UID: 1, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}}})
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
	err := executor.waitForContainerTask(context.Background(), "teacher-task", "pipeline", &types.StageRun{})
	if !errors.Is(err, context.Canceled) || !deleted.Load() {
		t.Fatalf("canceled pipeline left its task running: deleted=%v err=%v", deleted.Load(), err)
	}
}

func TestExecutionNodeUsesConfirmedWorker(t *testing.T) {
	saved := make(chan types.StageRun, 2)
	executor := NewStageExecutor(t.TempDir(), nil, nil, "runtime", &lifecycleStageRepo{saved: saved}, nil)
	stage := &types.StageRun{ID: "stage"}
	if err := executor.recordTaskWorker(context.Background(), stage, &gcsclient.TaskInfo{}); err != nil || stage.NodeName != "" || len(saved) != 0 {
		t.Fatalf("unallocated task acquired an invented node: %+v %v", stage, err)
	}
	task := &gcsclient.TaskInfo{TaskWorkers: []gcsclient.TaskWorker{{Name: "actual-worker"}}}
	if err := executor.recordTaskWorker(context.Background(), stage, task); err != nil {
		t.Fatal(err)
	}
	if actual := <-saved; actual.NodeName != "actual-worker" {
		t.Fatalf("actual worker was not persisted: %+v", actual)
	}
	task.TaskWorkers = append(task.TaskWorkers, gcsclient.TaskWorker{Name: "second-worker"})
	if err := executor.recordTaskWorker(context.Background(), stage, task); err == nil {
		t.Fatal("unexpected multi-node execution accepted")
	}
}

func TestControllerStageUsesCurrentHostname(t *testing.T) {
	executor := NewStageExecutor(t.TempDir(), nil, nil, "runtime", nil, nil)
	stage := &types.StageRun{ID: "stage", StageType: types.StageTeacherConfig}
	project := &types.Project{ID: "project", UID: 1, TeacherModelConfig: types.ModelConfig{ModelName: "teacher", Endpoint: "http://teacher/v1"}}
	pipeline := &types.PipelineRun{ID: "pipeline", ProjectID: "project", UID: 1, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}}}
	if err := executor.ExecuteStage(context.Background(), stage, pipeline, project); err != nil {
		t.Fatal(err)
	}
	hostname, err := os.Hostname()
	if err != nil || stage.NodeName != hostname {
		t.Fatalf("incorrect controller node: %q (hostname %q, err %v)", stage.NodeName, hostname, err)
	}
}
