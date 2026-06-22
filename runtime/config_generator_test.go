package runtime

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestGenerateTeacherInferConfigAPI(t *testing.T) {
	gen := NewConfigGenerator("/shared/distill")
	project := &types.Project{
		ID: "project-1",
		TeacherModelConfig: types.ModelConfig{
			ProviderType: types.ProviderAPI,
			ModelName:    "remote-teacher",
			Endpoint:     "https://teacher.example/v1",
			APISecretRef: "secret-token",
			MaxTokens:    256,
			ExtraParams: map[string]interface{}{
				"stream":        true,
				"system_prompt": "You are a helpful teacher.",
			},
		},
		StudentModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelName:    "student",
			ModelPath:    "/models/student",
		},
	}

	data, err := gen.GenerateTeacherInferConfig(project, "run-1")
	if err != nil {
		t.Fatalf("GenerateTeacherInferConfig returned error: %v", err)
	}

	var got TeacherInferConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got.JobType != "kd_black_box_api" {
		t.Fatalf("JobType = %q", got.JobType)
	}
	if got.Dataset.InstructionPath == "" || got.Dataset.LabeledPath == "" || got.Dataset.Template == "" {
		t.Fatalf("dataset paths were not populated: %#v", got.Dataset)
	}
	if got.Inference["base_url"] != "https://teacher.example/v1" {
		t.Fatalf("base_url = %#v", got.Inference["base_url"])
	}
	if got.Inference["api_key"] != "secret-token" {
		t.Fatalf("api_key = %#v", got.Inference["api_key"])
	}
	if got.Inference["max_new_tokens"].(float64) != 256 {
		t.Fatalf("max_new_tokens = %#v", got.Inference["max_new_tokens"])
	}
}

func TestGenerateTeacherInferConfigLocal(t *testing.T) {
	gen := NewConfigGenerator("/shared/distill")
	project := &types.Project{
		ID: "project-1",
		TeacherModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelName:    "teacher",
			ModelPath:    "/models/teacher",
		},
		StudentModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelName:    "student",
			ModelPath:    "/models/student",
		},
	}

	data, err := gen.GenerateTeacherInferConfig(project, "run-1")
	if err != nil {
		t.Fatalf("GenerateTeacherInferConfig returned error: %v", err)
	}

	var got TeacherInferConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got.JobType != "kd_black_box_local" {
		t.Fatalf("JobType = %q", got.JobType)
	}
	if got.Models.Teacher != "/models/teacher" {
		t.Fatalf("teacher model = %q", got.Models.Teacher)
	}
	for _, key := range []string{"enable_chunked_prefill", "gpu_memory_utilization", "trust_remote_code", "enforce_eager", "max_model_len"} {
		if _, ok := got.Inference[key]; !ok {
			t.Fatalf("local inference missing %s: %#v", key, got.Inference)
		}
	}
}

func TestGenerateStudentTrainConfigUsesEasyDistillContract(t *testing.T) {
	gen := NewConfigGenerator("/shared/distill")
	project := &types.Project{
		ID: "project-1",
		TeacherModelConfig: types.ModelConfig{
			ProviderType: types.ProviderAPI,
			ModelName:    "teacher",
		},
		StudentModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelName:    "student",
			ModelPath:    "/models/student",
		},
	}
	pipeline := &types.PipelineRun{
		TrainingConfig: types.TrainingConfig{
			NumTrainEpochs:            2,
			PerDeviceTrainBatchSize:   1,
			GradientAccumulationSteps: 8,
			LearningRate:              3e-5,
			WeightDecay:               0.05,
			WarmupRatio:               0.1,
			LRSchedulerType:           "cosine",
			MaxLength:                 1024,
			SaveSteps:                 50,
			LoggingSteps:              5,
		},
	}

	data, err := gen.GenerateStudentTrainConfig(project, pipeline, "run-1")
	if err != nil {
		t.Fatalf("GenerateStudentTrainConfig returned error: %v", err)
	}

	var got StudentTrainConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got.JobType != "kd_black_box_train_only" {
		t.Fatalf("JobType = %q", got.JobType)
	}
	wantLabeled := filepath.Join("/shared/distill", "projects", "project-1", "runs", "run-1", "data", "filtered", "train.json")
	if got.Dataset.LabeledPath != wantLabeled {
		t.Fatalf("LabeledPath = %q, want %q", got.Dataset.LabeledPath, wantLabeled)
	}
	if got.Dataset.InstructionPath != "" {
		t.Fatalf("InstructionPath should not be used by EasyDistill train config: %q", got.Dataset.InstructionPath)
	}
	if got.Training.GradientAccumulationSteps != 8 || got.Training.MaxLength != 1024 {
		t.Fatalf("training config did not preserve frontend values: %#v", got.Training)
	}
}

func TestGenerateEvaluateConfigUsesDataEvalContract(t *testing.T) {
	gen := NewConfigGenerator("/shared/distill")
	project := &types.Project{
		ID: "project-1",
		EvaluationConfig: &types.EvaluationConfig{
			ExtraParams: map[string]interface{}{
				"base_url":       "https://judge.example/v1",
				"api_key":        "judge-token",
				"max_new_tokens": float64(4096),
			},
		},
	}

	data, err := gen.GenerateEvaluateConfig(project, "run-1")
	if err != nil {
		t.Fatalf("GenerateEvaluateConfig returned error: %v", err)
	}

	var got EvaluateConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got.JobType != "cot_eval_api" {
		t.Fatalf("JobType = %q", got.JobType)
	}
	if got.Dataset.InputPath == "" || got.Dataset.OutputPath == "" {
		t.Fatalf("evaluation dataset paths not populated: %#v", got.Dataset)
	}
	if got.Inference["base_url"] != "https://judge.example/v1" || got.Inference["api_key"] != "judge-token" {
		t.Fatalf("evaluation inference mismatch: %#v", got.Inference)
	}
}
