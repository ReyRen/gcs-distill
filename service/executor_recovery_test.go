package service

import (
	"context"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"testing"
)

type recoveryPipelines struct {
	mysqlrepo.PipelineRepository
	runs map[string]*types.PipelineRun
}

func (r recoveryPipelines) RecoverableIDs(context.Context) ([]string, error) {
	return []string{"queued", "interrupted"}, nil
}
func (r recoveryPipelines) GetByID(_ context.Context, id string) (*types.PipelineRun, error) {
	return r.runs[id], nil
}
func (r recoveryPipelines) UpdateStatus(_ context.Context, id string, status types.PipelineStatus, msg string) error {
	r.runs[id].Status = status
	r.runs[id].ErrorMessage = msg
	return nil
}

type recoveryStages struct {
	mysqlrepo.StageRepository
	stage *types.StageRun
}

func (r recoveryStages) ListByPipeline(_ context.Context, id string) ([]*types.StageRun, error) {
	if id == "interrupted" {
		return []*types.StageRun{r.stage}, nil
	}
	return nil, nil
}
func (r recoveryStages) Update(_ context.Context, s *types.StageRun) error { *r.stage = *s; return nil }

func TestRecoveryQueuesOnlyUnstartedTasksAndFailsInterruptedRun(t *testing.T) {
	rows := recoveryPipelines{runs: map[string]*types.PipelineRun{"queued": {ID: "queued", Status: types.StatusScheduled, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0"}}}, "interrupted": {ID: "interrupted", Status: types.StatusRunning}}}
	stage := &types.StageRun{ID: "stage", PipelineRunID: "interrupted", Status: types.StatusRunning}
	s := &executorService{pipelineRepo: rows, stageRepo: recoveryStages{stage: stage}, queue: make(chan string, 2)}
	s.recoverTasks(context.Background())
	if len(s.queue) != 1 || <-s.queue != "queued" {
		t.Fatal("incorrect recovered queue")
	}
	if rows.runs["interrupted"].Status != types.StatusFailed || stage.Status != types.StatusFailed || stage.FinishedAt == nil {
		t.Fatal("interrupted task remained active")
	}
}
