package service

import (
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
)

func TestExecutionRejectsMissingReorderedOrPreviouslyRunStages(t *testing.T) {
	for _, train := range []bool{false, true} {
		p := &types.PipelineRun{ID: "pipeline", CurrentStage: 1, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{TrainStudent: train}}}
		var stages []*types.StageRun
		for i, kind := range runtime.PipelineStageTypes(p) {
			stages = append(stages, &types.StageRun{PipelineRunID: p.ID, StageOrder: i + 1, StageType: kind, Status: types.StatusPending})
		}
		if err := validateExecutionStages(p, stages); err != nil {
			t.Fatal(err)
		}
		for _, broken := range [][]*types.StageRun{nil, stages[:len(stages)-1], append(append([]*types.StageRun{}, stages...), stages[0])} {
			if err := validateExecutionStages(p, broken); err == nil {
				t.Fatal("incomplete execution could become succeeded")
			}
		}
		stages[0], stages[1] = stages[1], stages[0]
		if err := validateExecutionStages(p, stages); err == nil {
			t.Fatal("reordered execution accepted")
		}
		stages[0], stages[1] = stages[1], stages[0]
		stages[0].Status = types.StatusSucceeded
		if err := validateExecutionStages(p, stages); err == nil {
			t.Fatal("previously run stage accepted as a new execution")
		}
	}
}
