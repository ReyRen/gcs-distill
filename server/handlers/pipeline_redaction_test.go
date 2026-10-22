package handlers

import (
	"encoding/json"
	"github.com/ReyRen/gcs-distill/internal/types"
	"strings"
	"testing"
)

func TestPipelineResponseDoesNotExposeSnapshotCredentials(t *testing.T) {
	p := &types.PipelineRun{TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{
		TeacherModel: &types.ModelConfig{APISecretRef: "private-teacher-key", ExtraParams: map[string]interface{}{"api_key": "private-extra-key"}},
		Parameters:   map[string]interface{}{"backend": map[string]interface{}{"token": "private-token"}},
	}}}
	redactPipelineCredentials(p)
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-") {
		t.Fatal(string(data))
	}
}
