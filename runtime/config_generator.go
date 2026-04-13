package runtime

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ReyRen/gcs-distill/internal/types"
)

// ConfigGenerator EasyDistill 配置生成器
type ConfigGenerator struct {
	workspaceRoot string // 共享存储根目录
}

// NewConfigGenerator 创建配置生成器
func NewConfigGenerator(workspaceRoot string) *ConfigGenerator {
	return &ConfigGenerator{
		workspaceRoot: workspaceRoot,
	}
}

// TeacherInferConfig 教师模型推理配置
type TeacherInferConfig struct {
	JobType   string                 `json:"job_type"`
	Dataset   TeacherDatasetConfig   `json:"dataset"`
	Inference InferenceConfig        `json:"inference"`
	Models    TeacherModelsConfig    `json:"models"`
}

type TeacherDatasetConfig struct {
	InstructionPath string `json:"instruction_path"`
	LabeledPath     string `json:"labeled_path"`
}

type InferenceConfig struct {
	Temperature   float64 `json:"temperature"`
	MaxNewTokens  int     `json:"max_new_tokens"`
}

type TeacherModelsConfig struct {
	Teacher string `json:"teacher"`
}

// StudentTrainConfig 学生模型训练配置
type StudentTrainConfig struct {
	JobType  string               `json:"job_type"`
	Dataset  StudentDatasetConfig `json:"dataset"`
	Models   StudentModelsConfig  `json:"models"`
	Training TrainingConfig       `json:"training"`
}

type StudentDatasetConfig struct {
	InstructionPath string `json:"instruction_path"`
	Template        string `json:"template"`
}

type StudentModelsConfig struct {
	Teacher string `json:"teacher"`
	Student string `json:"student"`
}

type TrainingConfig struct {
	OutputDir              string  `json:"output_dir"`
	NumTrainEpochs         int     `json:"num_train_epochs"`
	PerDeviceTrainBatchSize int    `json:"per_device_train_batch_size"`
	LearningRate           float64 `json:"learning_rate"`
	SaveSteps              int     `json:"save_steps"`
	WarmupSteps            int     `json:"warmup_steps,omitempty"`
	LoggingSteps           int     `json:"logging_steps,omitempty"`
}

// EvaluateConfig 评估配置
type EvaluateConfig struct {
	JobType string             `json:"job_type"`
	Dataset EvalDatasetConfig  `json:"dataset"`
	Models  EvalModelsConfig   `json:"models"`
	Output  EvalOutputConfig   `json:"output"`
}

type EvalDatasetConfig struct {
	TestPath string `json:"test_path"`
}

type EvalModelsConfig struct {
	ModelPath string `json:"model_path"`
}

type EvalOutputConfig struct {
	ResultPath string `json:"result_path"`
}

// GenerateTeacherInferConfig 生成教师模型推理配置
// 对应阶段3: teacher_infer
func (g *ConfigGenerator) GenerateTeacherInferConfig(
	project *types.Project,
	runID string,
) ([]byte, error) {
	// 教师模型配置
	teacherConfig := project.TeacherModelConfig
	if teacherConfig == nil {
		return nil, fmt.Errorf("教师模型配置为空")
	}

	// 构建配置
	config := TeacherInferConfig{
		JobType: "kd_black_box_local",
		Dataset: TeacherDatasetConfig{
			InstructionPath: "/workspace/data/seed/instructions.json",
			LabeledPath:     "/workspace/data/generated/labeled.json",
		},
		Inference: InferenceConfig{
			Temperature:  teacherConfig.Temperature,
			MaxNewTokens: teacherConfig.MaxTokens,
		},
		Models: TeacherModelsConfig{
			Teacher: teacherConfig.ModelName,
		},
	}

	// 如果有默认值，设置默认值
	if config.Inference.Temperature == 0 {
		config.Inference.Temperature = 0.8
	}
	if config.Inference.MaxNewTokens == 0 {
		config.Inference.MaxNewTokens = 512
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateStudentTrainConfig 生成学生模型训练配置
// 对应阶段5: student_train
func (g *ConfigGenerator) GenerateStudentTrainConfig(
	project *types.Project,
	pipeline *types.PipelineRun,
	runID string,
) ([]byte, error) {
	// 教师和学生模型配置
	teacherConfig := project.TeacherModelConfig
	studentConfig := project.StudentModelConfig
	if teacherConfig == nil || studentConfig == nil {
		return nil, fmt.Errorf("模型配置不完整")
	}

	// 训练配置
	trainConfig := pipeline.TrainingConfig
	if trainConfig == nil {
		return nil, fmt.Errorf("训练配置为空")
	}

	// 构建配置
	config := StudentTrainConfig{
		JobType: "kd_black_box_train_only",
		Dataset: StudentDatasetConfig{
			InstructionPath: "/workspace/data/filtered/train.json",
			Template:        "chat_template/chat_template_kd.jinja",
		},
		Models: StudentModelsConfig{
			Teacher: teacherConfig.ModelName,
			Student: studentConfig.ModelName,
		},
		Training: TrainingConfig{
			OutputDir:               "/workspace/models/checkpoints/",
			NumTrainEpochs:          trainConfig.NumTrainEpochs,
			PerDeviceTrainBatchSize: trainConfig.PerDeviceTrainBatchSize,
			LearningRate:            trainConfig.LearningRate,
			SaveSteps:               trainConfig.SaveSteps,
			WarmupSteps:             trainConfig.WarmupSteps,
			LoggingSteps:            100, // 默认值
		},
	}

	// 设置默认值
	if config.Training.NumTrainEpochs == 0 {
		config.Training.NumTrainEpochs = 3
	}
	if config.Training.PerDeviceTrainBatchSize == 0 {
		config.Training.PerDeviceTrainBatchSize = 4
	}
	if config.Training.LearningRate == 0 {
		config.Training.LearningRate = 2e-5
	}
	if config.Training.SaveSteps == 0 {
		config.Training.SaveSteps = 1000
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateEvaluateConfig 生成评估配置
// 对应阶段6: evaluate
func (g *ConfigGenerator) GenerateEvaluateConfig(
	project *types.Project,
	runID string,
) ([]byte, error) {
	config := EvaluateConfig{
		JobType: "cot_eval_api",
		Dataset: EvalDatasetConfig{
			TestPath: "/workspace/data/filtered/test.json",
		},
		Models: EvalModelsConfig{
			ModelPath: "/workspace/models/checkpoints/",
		},
		Output: EvalOutputConfig{
			ResultPath: "/workspace/eval/results.json",
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

// GetRunWorkspace 获取运行实例的工作空间路径
func (g *ConfigGenerator) GetRunWorkspace(projectID, runID string) string {
	return filepath.Join(g.workspaceRoot, "projects", projectID, "runs", runID)
}

// GetConfigPath 获取配置文件路径
func (g *ConfigGenerator) GetConfigPath(projectID, runID, stageName string) string {
	workspace := g.GetRunWorkspace(projectID, runID)
	return filepath.Join(workspace, "configs", fmt.Sprintf("%s.json", stageName))
}

// GetDataPath 获取数据目录路径
func (g *ConfigGenerator) GetDataPath(projectID, runID, subPath string) string {
	workspace := g.GetRunWorkspace(projectID, runID)
	return filepath.Join(workspace, "data", subPath)
}

// GetLogPath 获取日志目录路径
func (g *ConfigGenerator) GetLogPath(projectID, runID, stageName string) string {
	workspace := g.GetRunWorkspace(projectID, runID)
	return filepath.Join(workspace, "logs", stageName)
}
