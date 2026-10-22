package handlers

import (
	"encoding/json"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func cloneSource() *types.PipelineRun {
	return &types.PipelineRun{UID: 42, ProjectID: "project", DatasetID: "seed", TrainingConfig: types.TrainingConfig{NumTrainEpochs: 3, Distillation: &types.DistillationConfig{Name: "original", TrainStudent: true, TeacherModel: &types.ModelConfig{Endpoint: "http://old/v1", ModelName: "teacher", APISecretRef: "private", ExtraParams: map[string]any{"api_key": "private-extra"}}}}, ResourceRequest: types.ResourceRequest{SelectedResources: []types.SelectedResource{{NodeName: "worker", XPUIndices: []int{6}}}}}
}

func TestCloneEditsRemainWithinProjectAndDoNotMutateSource(t *testing.T) {
	source := cloneSource()
	patch := json.RawMessage(`{"num_train_epochs":0.5,"distillation":{"train_student":false,"teacher_model":{"api_secret_ref":"********","extra_params":{"api_key":"********"}}}}`)
	out, err := clonedConfiguration(source, cloneRequest{TrainingConfig: patch})
	if err != nil {
		t.Fatal(err)
	}
	if out.UID != 42 || out.ProjectID != "project" || out.TrainingConfig.NumTrainEpochs != .5 || out.TrainingConfig.Distillation.TrainStudent {
		t.Fatalf("edited snapshot not applied: %+v", out)
	}
	if source.TrainingConfig.NumTrainEpochs != 3 || source.TrainingConfig.Distillation.Name != "original" || !source.TrainingConfig.Distillation.TrainStudent {
		t.Fatal("source snapshot was mutated")
	}
	if out.TrainingConfig.Distillation.TeacherModel.APISecretRef != "private" || out.TrainingConfig.Distillation.TeacherModel.ExtraParams["api_key"] != "private-extra" {
		t.Fatal("unchanged teacher lost server-side credentials")
	}
	out.ResourceRequest.SelectedResources[0].XPUIndices[0] = 7
	if source.ResourceRequest.SelectedResources[0].XPUIndices[0] != 6 {
		t.Fatal("resource snapshot aliases original")
	}
}

func TestCloneNeverReusesSecretsForChangedTeacher(t *testing.T) {
	source := cloneSource()
	for _, patch := range []string{
		`{"distillation":{"teacher_model":{"endpoint":"http://new/v1","api_secret_ref":"********"}}}`,
		`{"distillation":{"teacher_model":{"endpoint":"http://new/v1","extra_params":{"api_key":"********"}}}}`,
	} {
		if _, err := clonedConfiguration(source, cloneRequest{TrainingConfig: json.RawMessage(patch)}); err == nil {
			t.Fatal("redacted credential accepted for changed endpoint")
		}
	}
	out, err := clonedConfiguration(source, cloneRequest{TrainingConfig: json.RawMessage(`{"distillation":{"teacher_model":{"endpoint":"http://new/v1"}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if teacher := out.TrainingConfig.Distillation.TeacherModel; teacher.APISecretRef != "" || len(teacher.ExtraParams) != 0 {
		t.Fatal("original credentials inherited by another endpoint")
	}
}
