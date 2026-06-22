package runtime

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ReyRen/gcs-distill/internal/types"
)

const (
	defaultTemplateRelPath = "chat_template/chat_template_kd.jinja"
	defaultDatasetSeed     = 42
)

type ConfigGenerator struct {
	workspaceRoot string
}

func NewConfigGenerator(workspaceRoot string) *ConfigGenerator {
	return &ConfigGenerator{workspaceRoot: workspaceRoot}
}

type TeacherInferConfig struct {
	JobType   string                 `json:"job_type"`
	Dataset   EasyDistillDataset     `json:"dataset"`
	Inference map[string]interface{} `json:"inference"`
	Models    EasyDistillModels      `json:"models"`
}

type StudentTrainConfig struct {
	JobType  string              `json:"job_type"`
	Dataset  EasyDistillDataset  `json:"dataset"`
	Models   EasyDistillModels   `json:"models"`
	Training EasyDistillTraining `json:"training"`
}

type EvaluateConfig struct {
	JobType   string                 `json:"job_type"`
	Dataset   EasyDistillDataset     `json:"dataset"`
	Inference map[string]interface{} `json:"inference"`
}

type EasyDistillDataset struct {
	InstructionPath string `json:"instruction_path,omitempty"`
	LabeledPath     string `json:"labeled_path,omitempty"`
	InputPath       string `json:"input_path,omitempty"`
	OutputPath      string `json:"output_path,omitempty"`
	Template        string `json:"template,omitempty"`
	Seed            int    `json:"seed,omitempty"`
}

type EasyDistillModels struct {
	Teacher string `json:"teacher,omitempty"`
	Student string `json:"student,omitempty"`
}

type EasyDistillTraining struct {
	OutputDir                 string  `json:"output_dir"`
	NumTrainEpochs            int     `json:"num_train_epochs"`
	PerDeviceTrainBatchSize   int     `json:"per_device_train_batch_size"`
	GradientAccumulationSteps int     `json:"gradient_accumulation_steps,omitempty"`
	MaxLength                 int     `json:"max_length,omitempty"`
	SaveSteps                 int     `json:"save_steps,omitempty"`
	LoggingSteps              int     `json:"logging_steps,omitempty"`
	LearningRate              float64 `json:"learning_rate"`
	WeightDecay               float64 `json:"weight_decay,omitempty"`
	WarmupRatio               float64 `json:"warmup_ratio,omitempty"`
	LRSchedulerType           string  `json:"lr_scheduler_type,omitempty"`
}

func (g *ConfigGenerator) GenerateTeacherInferConfig(project *types.Project, runID string) ([]byte, error) {
	teacher := project.TeacherModelConfig
	if teacher.ModelName == "" {
		return nil, fmt.Errorf("teacher model name is required")
	}

	jobType, err := teacherJobType(teacher.ProviderType)
	if err != nil {
		return nil, err
	}

	workspace := g.GetRunWorkspace(project.ID, runID)
	config := TeacherInferConfig{
		JobType: jobType,
		Dataset: EasyDistillDataset{
			InstructionPath: filepath.Join(workspace, "data", "seed", "instructions.json"),
			LabeledPath:     filepath.Join(workspace, "data", "generated", "labeled.json"),
			Template:        g.GetTemplatePath(project.ID, runID),
			Seed:            defaultDatasetSeed,
		},
		Inference: buildTeacherInference(teacher),
		Models: EasyDistillModels{
			Teacher: modelPathOrName(teacher),
			Student: modelPathOrName(project.StudentModelConfig),
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

func (g *ConfigGenerator) GenerateStudentTrainConfig(
	project *types.Project,
	pipeline *types.PipelineRun,
	runID string,
) ([]byte, error) {
	student := project.StudentModelConfig
	if student.ModelName == "" {
		return nil, fmt.Errorf("student model name is required")
	}
	if modelPathOrName(student) == "" {
		return nil, fmt.Errorf("student model path is required")
	}

	train := withTrainingDefaults(pipeline.TrainingConfig)
	workspace := g.GetRunWorkspace(project.ID, runID)
	config := StudentTrainConfig{
		JobType: "kd_black_box_train_only",
		Dataset: EasyDistillDataset{
			LabeledPath: filepath.Join(workspace, "data", "filtered", "train.json"),
			Template:    g.GetTemplatePath(project.ID, runID),
			Seed:        defaultDatasetSeed,
		},
		Models: EasyDistillModels{
			Teacher: modelPathOrName(project.TeacherModelConfig),
			Student: modelPathOrName(student),
		},
		Training: EasyDistillTraining{
			OutputDir:                 filepath.Join(workspace, "models", "checkpoints"),
			NumTrainEpochs:            train.NumTrainEpochs,
			PerDeviceTrainBatchSize:   train.PerDeviceTrainBatchSize,
			GradientAccumulationSteps: train.GradientAccumulationSteps,
			MaxLength:                 train.MaxLength,
			SaveSteps:                 train.SaveSteps,
			LoggingSteps:              train.LoggingSteps,
			LearningRate:              train.LearningRate,
			WeightDecay:               train.WeightDecay,
			WarmupRatio:               train.WarmupRatio,
			LRSchedulerType:           train.LRSchedulerType,
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

func (g *ConfigGenerator) GenerateEvaluateConfig(project *types.Project, runID string) ([]byte, error) {
	inference, err := buildEvaluationInference(project)
	if err != nil {
		return nil, err
	}

	workspace := g.GetRunWorkspace(project.ID, runID)
	config := EvaluateConfig{
		JobType: "cot_eval_api",
		Dataset: EasyDistillDataset{
			InputPath:  filepath.Join(workspace, "data", "filtered", "test.json"),
			OutputPath: filepath.Join(workspace, "eval", "results.json"),
		},
		Inference: inference,
	}

	return json.MarshalIndent(config, "", "  ")
}

func teacherJobType(provider types.ProviderType) (string, error) {
	switch provider {
	case types.ProviderAPI:
		return "kd_black_box_api", nil
	case types.ProviderLocal:
		return "kd_black_box_local", nil
	default:
		return "", fmt.Errorf("unsupported teacher provider_type: %s", provider)
	}
}

func buildTeacherInference(config types.ModelConfig) map[string]interface{} {
	inference := map[string]interface{}{
		"max_new_tokens": defaultInt(config.MaxTokens, 512),
	}

	if config.ProviderType == types.ProviderAPI {
		inference["base_url"] = config.Endpoint
		inference["api_key"] = firstString(extraString(config, "api_key"), config.APISecretRef)
		inference["stream"] = extraBool(config, "stream", false)
		inference["system_prompt"] = extraString(config, "system_prompt")
	} else {
		inference["enable_chunked_prefill"] = extraBool(config, "enable_chunked_prefill", true)
		inference["seed"] = extraInt(config, "seed", 777)
		inference["gpu_memory_utilization"] = extraFloat(config, "gpu_memory_utilization", 0.9)
		inference["temperature"] = defaultFloat(config.Temperature, 0.8)
		inference["trust_remote_code"] = extraBool(config, "trust_remote_code", true)
		inference["enforce_eager"] = extraBool(config, "enforce_eager", false)
		inference["max_model_len"] = extraInt(config, "max_model_len", 4096)
	}

	mergeNamedExtraMap(inference, config.ExtraParams, "inference")
	return inference
}

func buildEvaluationInference(project *types.Project) (map[string]interface{}, error) {
	baseURL := ""
	apiKey := ""
	maxTokens := 8196

	if project.EvaluationConfig != nil {
		baseURL = stringFromMap(project.EvaluationConfig.ExtraParams, "base_url")
		apiKey = stringFromMap(project.EvaluationConfig.ExtraParams, "api_key")
		if value, ok := intFromMap(project.EvaluationConfig.ExtraParams, "max_new_tokens"); ok {
			maxTokens = value
		}
	}
	if baseURL == "" && project.TeacherModelConfig.ProviderType == types.ProviderAPI {
		baseURL = project.TeacherModelConfig.Endpoint
	}
	if apiKey == "" && project.TeacherModelConfig.ProviderType == types.ProviderAPI {
		apiKey = firstString(extraString(project.TeacherModelConfig, "api_key"), project.TeacherModelConfig.APISecretRef)
	}
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("evaluation_config.extra_params.base_url and api_key are required for EasyDistill evaluation")
	}

	return map[string]interface{}{
		"base_url":       baseURL,
		"api_key":        apiKey,
		"max_new_tokens": maxTokens,
	}, nil
}

func withTrainingDefaults(config types.TrainingConfig) types.TrainingConfig {
	if config.NumTrainEpochs == 0 {
		config.NumTrainEpochs = 3
	}
	if config.PerDeviceTrainBatchSize == 0 {
		config.PerDeviceTrainBatchSize = 4
	}
	if config.LearningRate == 0 {
		config.LearningRate = 2e-5
	}
	if config.SaveSteps == 0 {
		config.SaveSteps = 1000
	}
	if config.LoggingSteps == 0 {
		config.LoggingSteps = 100
	}
	if config.GradientAccumulationSteps == 0 {
		config.GradientAccumulationSteps = 1
	}
	if config.MaxLength == 0 {
		config.MaxLength = 512
	}
	return config
}

func modelPathOrName(config types.ModelConfig) string {
	if config.ModelPath != "" {
		return config.ModelPath
	}
	if path := extraString(config, "model_path"); path != "" {
		return path
	}
	return config.ModelName
}

func mergeNamedExtraMap(target map[string]interface{}, extra map[string]interface{}, key string) {
	value, ok := extra[key]
	if !ok {
		return
	}
	items, ok := value.(map[string]interface{})
	if !ok {
		return
	}
	for k, v := range items {
		target[k] = v
	}
}

func extraString(config types.ModelConfig, key string) string {
	return stringFromMap(config.ExtraParams, key)
}

func extraBool(config types.ModelConfig, key string, fallback bool) bool {
	value, ok := config.ExtraParams[key]
	if !ok {
		return fallback
	}
	v, ok := value.(bool)
	if !ok {
		return fallback
	}
	return v
}

func extraInt(config types.ModelConfig, key string, fallback int) int {
	if value, ok := intFromMap(config.ExtraParams, key); ok {
		return value
	}
	return fallback
}

func extraFloat(config types.ModelConfig, key string, fallback float64) float64 {
	value, ok := config.ExtraParams[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	default:
		return fallback
	}
}

func stringFromMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func intFromMap(values map[string]interface{}, key string) (int, bool) {
	if values == nil {
		return 0, false
	}
	value, ok := values[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultFloat(value, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (g *ConfigGenerator) GetRunWorkspace(projectID, runID string) string {
	return filepath.Join(g.workspaceRoot, "projects", projectID, "runs", runID)
}

func (g *ConfigGenerator) GetConfigPath(projectID, runID, stageName string) string {
	return filepath.Join(g.GetRunWorkspace(projectID, runID), "configs", fmt.Sprintf("%s.json", stageName))
}

func (g *ConfigGenerator) GetDataPath(projectID, runID, subPath string) string {
	return filepath.Join(g.GetRunWorkspace(projectID, runID), "data", subPath)
}

func (g *ConfigGenerator) GetLogPath(projectID, runID, stageName string) string {
	return filepath.Join(g.GetRunWorkspace(projectID, runID), "logs", stageName)
}

func (g *ConfigGenerator) GetTemplatePath(projectID, runID string) string {
	return filepath.Join(g.GetRunWorkspace(projectID, runID), defaultTemplateRelPath)
}
