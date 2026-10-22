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
		if stage["stage"] != "agent_trajectory" && stage["stage"] != "build_sft" && config["system_prompt"] != cfg["generation"].(map[string]interface{})["system_prompt"] {
			t.Fatal("Agent prompt stage ignored generation system prompt")
		}
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

func TestAgentAndVisionDefaultsKeepLongPromptsTrainable(t *testing.T) {
	for _, job := range []string{"agent_distill", "mm_instruct_distill", "mm_cot_distill"} {
		for _, requested := range []int{0, 1024} {
			p := &types.PipelineRun{UID: 42, ID: "run", TrainingConfig: types.TrainingConfig{MaxLength: requested,
				Distillation: &types.DistillationConfig{JobType: job, TrainStudent: true,
					StudentModel: &types.ModelConfig{ProviderType: types.ProviderLocal, ModelPath: "/models/student"},
					TrainerImage: "trainer", ModelTemplate: "qwen"}},
				ResourceRequest: types.ResourceRequest{SelectedResources: []types.SelectedResource{{NodeName: "worker", XPUIndices: []int{7}}}}}
			if err := NormalizeDistillation(p, project2()); err != nil {
				t.Fatal(err)
			}
			expected := requested
			if expected == 0 {
				expected = 4096
			}
			config, _, err := NewConfigGenerator(t.TempDir()).BuildTrainerConfig(project2(), p)
			if err != nil || p.TrainingConfig.MaxLength != expected || config["cutoff_len"] != expected {
				t.Fatal("long-form default or explicit length ignored", job, config, err)
			}
		}
	}
}
