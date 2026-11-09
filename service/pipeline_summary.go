package service

import (
	"context"
	"strconv"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/ReyRen/gcs-distill/runtime"
)

func (s *pipelineService) enrichPipeline(ctx context.Context, p *types.PipelineRun) {
	w := p.TrainingConfig.Distillation
	summary := &types.PipelineSummary{}
	p.Summary = summary
	if w == nil {
		return
	}
	p.Name = w.Name
	summary.JobType = w.JobType
	summary.TrainStudent = w.TrainStudent
	if recipe, ok := runtime.FindWorkflow(w.JobType); ok {
		summary.OutputFormat = recipe.OutputFormat
	}
	stages, err := s.stageRepo.ListByPipeline(ctx, p.ID)
	if err != nil {
		return
	}
	for _, stage := range stages {
		switch stage.StageType {
		case types.StageDistillData:
			summary.Expansion = stage.Metrics["expansion"]
			summary.Generation = stage.Metrics["generation"]
		case types.StageDatasetBuild:
			summary.InputRecords, _ = strconv.Atoi(stage.OutputManifest["record_count"])
			summary.SourceRecords, _ = strconv.Atoi(stage.OutputManifest["source_records"])
			if summary.SourceRecords == 0 {
				summary.SourceRecords = summary.InputRecords
			}
		case types.StageDatasetExport:
			summary.DataSplit = stage.Metrics["data_split"]
			if stage.Status == types.StatusSucceeded {
				summary.OutputRecords, _ = strconv.Atoi(stage.OutputManifest["record_count"])
				summary.OutputDatasetID = stage.OutputManifest["dataset_id"]
			}
		case types.StageStudentTrain:
			summary.HasModelWeights = stage.Status == types.StatusSucceeded && stage.OutputManifest["checkpoint_path"] != ""
		}
	}
}
