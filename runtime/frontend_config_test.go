package runtime

import (
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestFrontendGenerationControlsOverrideStageDefaults(t *testing.T) {
	project := project2()
	p := &types.PipelineRun{UID: 42, ID: "run", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: "agent_distill", TeacherModel: &types.ModelConfig{ModelName: "teacher", Endpoint: "http://teacher/v1", MaxTokens: 3000, Concurrency: 2}}}}
	if err := NormalizeDistillation(p, project); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfigGenerator(t.TempDir()).GenerateDistill2Config(project, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range cfg["pipeline"].([]interface{}) {
		stage := raw.(map[string]interface{})
		config := stage["config"].(map[string]interface{})
		if workers, exists := config["max_workers"]; exists && workers != 2 {
			t.Fatalf("stage %s ignored concurrency: %v", stage["stage"], workers)
		}
		if tokens, exists := config["max_tokens"]; exists && tokens != 3000 {
			t.Fatalf("stage %s ignored output length: %v", stage["stage"], tokens)
		}
		if stage["stage"] == "agent_trajectory" && config["solve"].(map[string]interface{})["max_tokens"] != 3000 {
			t.Fatal("nested trajectory generation ignored output length")
		}
	}
}
