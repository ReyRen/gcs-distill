package types

import "time"

// PipelineStatus is the execution state for a pipeline or stage.
type PipelineStatus string

const (
	StatusPending   PipelineStatus = "pending"
	StatusScheduled PipelineStatus = "scheduled"
	StatusPreparing PipelineStatus = "preparing"
	StatusRunning   PipelineStatus = "running"
	StatusSucceeded PipelineStatus = "succeeded"
	StatusFailed    PipelineStatus = "failed"
	StatusCanceled  PipelineStatus = "canceled"
)

// StageType is a distillation pipeline stage.
type StageType string

const (
	StageTeacherConfig StageType = "teacher_config"
	StageDatasetBuild  StageType = "dataset_build"
	StageTeacherInfer  StageType = "teacher_infer"
	StageDataGovern    StageType = "data_govern"
	StageStudentTrain  StageType = "student_train"
	StageEvaluate      StageType = "evaluate"
)

// ProviderType is the source type for a model.
type ProviderType string

const (
	ProviderAPI   ProviderType = "api"
	ProviderLocal ProviderType = "local"
)

// Project is a distillation project.
type Project struct {
	ID                 string            `json:"id" db:"id"`
	UID                int               `json:"uid" db:"uid"`
	Name               string            `json:"name" db:"name"`
	Description        string            `json:"description" db:"description"`
	BusinessScenario   string            `json:"business_scenario" db:"business_scenario"`
	TeacherModelConfig ModelConfig       `json:"teacher_model_config" db:"teacher_model_config"`     // JSON
	StudentModelConfig ModelConfig       `json:"student_model_config" db:"student_model_config"`     // JSON
	EvaluationConfig   *EvaluationConfig `json:"evaluation_config,omitempty" db:"evaluation_config"` // JSON
	CreatedAt          time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at" db:"updated_at"`
}

// ModelConfig describes a teacher or student model.
type ModelConfig struct {
	ProviderType   ProviderType           `json:"provider_type"`
	ModelID        string                 `json:"model_id,omitempty"`
	ModelName      string                 `json:"model_name"`
	ModelPath      string                 `json:"model_path,omitempty"`
	Endpoint       string                 `json:"endpoint,omitempty"`
	APISecretRef   string                 `json:"api_secret_ref,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	MaxTokens      int                    `json:"max_tokens,omitempty"`
	Concurrency    int                    `json:"concurrency,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
	ExtraParams    map[string]interface{} `json:"extra_params,omitempty"`
}

// EvaluationConfig configures distillation evaluation.
type EvaluationConfig struct {
	Metrics      []string               `json:"metrics"`
	TestSetRatio float64                `json:"test_set_ratio"`
	ExtraParams  map[string]interface{} `json:"extra_params,omitempty"`
}

// Dataset is a reusable distillation dataset.
type Dataset struct {
	ID          string    `json:"id" db:"id"`
	UID         int       `json:"uid" db:"uid"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	SourceType  string    `json:"source_type" db:"source_type"`
	FilePath    string    `json:"file_path" db:"file_path"`
	RecordCount int       `json:"record_count" db:"record_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// PipelineRun is one distillation pipeline execution.
type PipelineRun struct {
	ID              string          `json:"id" db:"id"`
	UID             int             `json:"uid" db:"uid"`
	ProjectID       string          `json:"project_id" db:"project_id"`
	DatasetID       string          `json:"dataset_id" db:"dataset_id"`
	Status          PipelineStatus  `json:"status" db:"status"`
	CurrentStage    int             `json:"current_stage" db:"current_stage"`
	TriggerMode     string          `json:"trigger_mode" db:"trigger_mode"`
	TrainingConfig  TrainingConfig  `json:"training_config" db:"training_config"`   // JSON
	ResourceRequest ResourceRequest `json:"resource_request" db:"resource_request"` // JSON
	ErrorMessage    string          `json:"error_message,omitempty" db:"error_message"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty" db:"started_at"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty" db:"finished_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

// TrainingConfig configures student training.
type TrainingConfig struct {
	NumTrainEpochs            int     `json:"num_train_epochs"`
	PerDeviceTrainBatchSize   int     `json:"per_device_train_batch_size"`
	GradientAccumulationSteps int     `json:"gradient_accumulation_steps,omitempty"`
	LearningRate              float64 `json:"learning_rate"`
	WeightDecay               float64 `json:"weight_decay,omitempty"`
	WarmupRatio               float64 `json:"warmup_ratio,omitempty"`
	LRSchedulerType           string  `json:"lr_scheduler_type,omitempty"`
	SaveSteps                 int     `json:"save_steps,omitempty"`
	LoggingSteps              int     `json:"logging_steps,omitempty"`
	MaxLength                 int     `json:"max_length,omitempty"`
}

// ResourceRequest describes pipeline resource requirements.
type ResourceRequest struct {
	GPUCount          int                `json:"gpu_count"`
	GPUDeviceIDs      string             `json:"gpu_device_ids,omitempty"`
	GPUType           string             `json:"gpu_type,omitempty"`
	MemoryGB          int                `json:"memory_gb,omitempty"`
	CPUCores          int                `json:"cpu_cores,omitempty"`
	SelectedResources []SelectedResource `json:"selected_resources,omitempty"`
}

// SelectedResource is an explicit gcs-v2 node and XPU selection.
type SelectedResource struct {
	NodeName    string `json:"node_name"`
	NodeAddress string `json:"node_address"`
	XPUIndices  []int  `json:"xpu_indices"`
}

// StageRun is one pipeline stage execution.
type StageRun struct {
	ID             string                 `json:"id" db:"id"`
	PipelineRunID  string                 `json:"pipeline_run_id" db:"pipeline_run_id"`
	StageType      StageType              `json:"stage_type" db:"stage_type"`
	StageOrder     int                    `json:"stage_order" db:"stage_order"`
	Status         PipelineStatus         `json:"status" db:"status"`
	ContainerID    string                 `json:"container_id,omitempty" db:"container_id"`
	NodeName       string                 `json:"node_name,omitempty" db:"node_name"`
	ConfigPath     string                 `json:"config_path,omitempty" db:"config_path"`
	InputManifest  map[string]string      `json:"input_manifest,omitempty" db:"input_manifest"`   // JSON
	OutputManifest map[string]string      `json:"output_manifest,omitempty" db:"output_manifest"` // JSON
	Metrics        map[string]interface{} `json:"metrics,omitempty" db:"metrics"`                 // JSON
	LogPath        string                 `json:"log_path,omitempty" db:"log_path"`
	RetryCount     int                    `json:"retry_count" db:"retry_count"`
	ErrorMessage   string                 `json:"error_message,omitempty" db:"error_message"`
	StartedAt      *time.Time             `json:"started_at,omitempty" db:"started_at"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}

// ContainerRun is a persisted container execution record.
type ContainerRun struct {
	ID            string            `json:"id" db:"id"`
	StageRunID    string            `json:"stage_run_id" db:"stage_run_id"`
	ContainerName string            `json:"container_name" db:"container_name"`
	Image         string            `json:"image" db:"image"`
	NodeName      string            `json:"node_name" db:"node_name"`
	NodeAddr      string            `json:"node_addr" db:"node_addr"`
	Command       string            `json:"command" db:"command"`
	Args          []string          `json:"args" db:"args"`               // JSON
	Envs          map[string]string `json:"envs,omitempty" db:"envs"`     // JSON
	Mounts        []Mount           `json:"mounts,omitempty" db:"mounts"` // JSON
	XPUAllocation string            `json:"xpu_allocation,omitempty" db:"xpu_allocation"`
	ExitCode      *int              `json:"exit_code,omitempty" db:"exit_code"`
	StartedAt     *time.Time        `json:"started_at,omitempty" db:"started_at"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

// Mount is a container bind mount.
type Mount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

// EvaluationReport is a distillation evaluation result.
type EvaluationReport struct {
	ID            string                 `json:"id" db:"id"`
	PipelineRunID string                 `json:"pipeline_run_id" db:"pipeline_run_id"`
	StageRunID    string                 `json:"stage_run_id" db:"stage_run_id"`
	Metrics       map[string]float64     `json:"metrics" db:"metrics"`           // JSON
	Details       map[string]interface{} `json:"details,omitempty" db:"details"` // JSON
	Summary       string                 `json:"summary,omitempty" db:"summary"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
}
