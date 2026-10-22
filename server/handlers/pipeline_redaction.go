package handlers

import "github.com/ReyRen/gcs-distill/internal/types"

func redactProjectCredentials(p *types.Project) {
	if p.TeacherModelConfig.APISecretRef != "" {
		p.TeacherModelConfig.APISecretRef = "********"
	}
	redactConfig(p.TeacherModelConfig.ExtraParams)
}

// Repository writes finish before handlers redact response-only objects.
func redactPipelineCredentials(p *types.PipelineRun) {
	if p == nil || p.TrainingConfig.Distillation == nil {
		return
	}
	w := p.TrainingConfig.Distillation
	for _, model := range []*types.ModelConfig{w.TeacherModel, w.StudentModel} {
		if model == nil {
			continue
		}
		if model.APISecretRef != "" {
			model.APISecretRef = "********"
		}
		redactConfig(model.ExtraParams)
	}
	redactConfig(w.Parameters)
}
