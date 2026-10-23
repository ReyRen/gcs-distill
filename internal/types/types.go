package types

import "time"

type ListFilter struct{ Name, Status, SourceType, JobType string }

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
	StageStudentTrain  StageType = "student_train"
	StageDistillData   StageType = "distill_data"
	StageDatasetExport StageType = "dataset_export"
)

// ProviderType is the source type for a model.
type ProviderType string

const (
	ProviderAPI   ProviderType = "api"
	ProviderLocal ProviderType = "local"
)

// Project is a distillation project.
type Project struct {
	ID                 string      `json:"id" db:"id"`
	UID                int         `json:"uid" db:"uid"`
	Name               string      `json:"name" db:"name"`
	Description        string      `json:"description" db:"description"`
	BusinessScenario   string      `json:"business_scenario" db:"business_scenario"`
	TeacherModelConfig ModelConfig `json:"teacher_model_config" db:"teacher_model_config"` // JSON
	CreatedAt          time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at" db:"updated_at"`
}

// ModelConfig describes a teacher or student model.
type ModelConfig struct {
	ServiceID      string                 `json:"service_id,omitempty"`
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

// Dataset is a reusable distillation dataset.
type Dataset struct {
	Lineage     map[string]string `json:"lineage,omitempty" db:"-"`
	ID          string            `json:"id" db:"id"`
	UID         int               `json:"uid" db:"uid"`
	Name        string            `json:"name" db:"name"`
	DatasetName string            `json:"dataset_name,omitempty" db:"-"`
	Description string            `json:"description" db:"description"`
	SourceType  string            `json:"source_type" db:"source_type"`
	FilePath    string            `json:"file_path" db:"file_path"`
	RecordCount int               `json:"record_count" db:"record_count"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
}

// PipelineRun is one distillation pipeline execution.
type PipelineRun struct {
	Name            string           `json:"name" db:"-"`
	Summary         *PipelineSummary `json:"summary,omitempty" db:"-"`
	ID              string           `json:"id" db:"id"`
	UID             int              `json:"uid" db:"uid"`
	ProjectID       string           `json:"project_id" db:"project_id"`
	DatasetID       string           `json:"dataset_id" db:"dataset_id"`
	Status          PipelineStatus   `json:"status" db:"status"`
	CurrentStage    int              `json:"current_stage" db:"current_stage"`
	TrainingConfig  TrainingConfig   `json:"training_config" db:"training_config"`   // JSON
	ResourceRequest ResourceRequest  `json:"resource_request" db:"resource_request"` // JSON
	ErrorMessage    string           `json:"error_message,omitempty" db:"error_message"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	StartedAt       *time.Time       `json:"started_at,omitempty" db:"started_at"`
	FinishedAt      *time.Time       `json:"finished_at,omitempty" db:"finished_at"`
	UpdatedAt       time.Time        `json:"updated_at" db:"updated_at"`
}

type PipelineSummary struct {
	JobType         string `json:"job_type"`
	OutputFormat    string `json:"output_format"`
	TrainStudent    bool   `json:"train_student"`
	InputRecords    int    `json:"input_records"`
	OutputRecords   int    `json:"output_records"`
	OutputDatasetID string `json:"output_dataset_id,omitempty"`
	HasModelWeights bool   `json:"has_model_weights"`
}

// TrainingConfig configures student training.
type TrainingConfig struct {
	// Each run owns its immutable workflow and model snapshot.
	Distillation              *DistillationConfig `json:"distillation,omitempty"`
	NumTrainEpochs            float64             `json:"num_train_epochs"`
	PerDeviceTrainBatchSize   int                 `json:"per_device_train_batch_size"`
	GradientAccumulationSteps int                 `json:"gradient_accumulation_steps,omitempty"`
	LearningRate              float64             `json:"learning_rate"`
	WeightDecay               float64             `json:"weight_decay,omitempty"`
	WarmupRatio               float64             `json:"warmup_ratio,omitempty"`
	LRSchedulerType           string              `json:"lr_scheduler_type,omitempty"`
	SaveSteps                 int                 `json:"save_steps,omitempty"`
	LoggingSteps              int                 `json:"logging_steps,omitempty"`
	MaxLength                 int                 `json:"max_length,omitempty"`
}

// DistillationConfig describes an EasyDistill 2 data workflow and optional training.
type DistillationConfig struct {
	FieldMapping   map[string]string      `json:"field_mapping,omitempty"`
	TeacherModel   *ModelConfig           `json:"teacher_model,omitempty"`
	StudentModel   *ModelConfig           `json:"student_model,omitempty"`
	Name           string                 `json:"name,omitempty"`
	Version        string                 `json:"version"`
	JobType        string                 `json:"job_type"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	TrainStudent   bool                   `json:"train_student"`
	TrainerImage   string                 `json:"trainer_image,omitempty"`
	ModelTemplate  string                 `json:"model_template,omitempty"`
	FinetuningType string                 `json:"finetuning_type,omitempty"`
}

// ResourceRequest describes pipeline resource requirements.
type ResourceRequest struct {
	ExecutorNodes     []string           `json:"executor_nodes,omitempty"`
	GPUCount          int                `json:"gpu_count"`
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
	ErrorMessage   string                 `json:"error_message,omitempty" db:"error_message"`
	StartedAt      *time.Time             `json:"started_at,omitempty" db:"started_at"`
	FinishedAt     *time.Time             `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}
