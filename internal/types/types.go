package types

import "time"

// PipelineStatus 流水线状态
type PipelineStatus string

const (
	StatusPending    PipelineStatus = "pending"     // 等待中
	StatusScheduled  PipelineStatus = "scheduled"   // 已调度
	StatusPreparing  PipelineStatus = "preparing"   // 准备中
	StatusRunning    PipelineStatus = "running"     // 运行中
	StatusSucceeded  PipelineStatus = "succeeded"   // 成功
	StatusFailed     PipelineStatus = "failed"      // 失败
	StatusCanceled   PipelineStatus = "canceled"    // 已取消
)

// StageType 阶段类型
type StageType string

const (
	StageTeacherConfig  StageType = "teacher_config"   // 教师模型配置
	StageDatasetBuild   StageType = "dataset_build"    // 蒸馏数据构建
	StageTeacherInfer   StageType = "teacher_infer"    // 教师推理与样本生成
	StageDataGovern     StageType = "data_govern"      // 蒸馏数据治理
	StageStudentTrain   StageType = "student_train"    // 学生模型训练
	StageEvaluate       StageType = "evaluate"         // 蒸馏效果评估
)

// ProviderType 模型提供者类型
type ProviderType string

const (
	ProviderAPI   ProviderType = "api"    // API 型教师模型
	ProviderLocal ProviderType = "local"  // 本地教师模型
)

// Project 蒸馏项目
type Project struct {
	ID                   string            `json:"id" db:"id"`
	Name                 string            `json:"name" db:"name"`
	Description          string            `json:"description" db:"description"`
	BusinessScenario     string            `json:"business_scenario" db:"business_scenario"`
	TeacherModelConfig   ModelConfig       `json:"teacher_model_config" db:"teacher_model_config"` // JSONB
	StudentModelConfig   ModelConfig       `json:"student_model_config" db:"student_model_config"` // JSONB
	EvaluationConfig     *EvaluationConfig `json:"evaluation_config,omitempty" db:"evaluation_config"` // JSONB
	CreatedAt            time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at" db:"updated_at"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	ProviderType     ProviderType      `json:"provider_type"`
	ModelName        string            `json:"model_name"`
	ModelPath        string            `json:"model_path,omitempty"`       // 本地模型文件路径 (仅用于 local 类型)
	Endpoint         string            `json:"endpoint,omitempty"`         // API 端点
	APISecretRef     string            `json:"api_secret_ref,omitempty"`   // API 密钥引用
	Temperature      float64           `json:"temperature,omitempty"`
	MaxTokens        int               `json:"max_tokens,omitempty"`
	Concurrency      int               `json:"concurrency,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds,omitempty"`
	ExtraParams      map[string]interface{} `json:"extra_params,omitempty"`
}

// EvaluationConfig 评估配置
type EvaluationConfig struct {
	Metrics      []string          `json:"metrics"`       // ["bleu", "rouge", "accuracy"]
	TestSetRatio float64           `json:"test_set_ratio"` // 测试集比例
	ExtraParams  map[string]interface{} `json:"extra_params,omitempty"`
}

// Dataset 数据集
type Dataset struct {
	ID          string    `json:"id" db:"id"`
	ProjectID   string    `json:"project_id" db:"project_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	SourceType  string    `json:"source_type" db:"source_type"` // "upload", "import", "generated"
	FilePath    string    `json:"file_path" db:"file_path"`
	RecordCount int       `json:"record_count" db:"record_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// PipelineRun 流水线运行实例
type PipelineRun struct {
	ID               string            `json:"id" db:"id"`
	ProjectID        string            `json:"project_id" db:"project_id"`
	DatasetID        string            `json:"dataset_id" db:"dataset_id"`
	Status           PipelineStatus    `json:"status" db:"status"`
	CurrentStage     int               `json:"current_stage" db:"current_stage"` // 当前阶段序号 (1-6)
	TriggerMode      string            `json:"trigger_mode" db:"trigger_mode"`   // "manual", "scheduled"
	TrainingConfig   TrainingConfig    `json:"training_config" db:"training_config"` // JSONB
	ResourceRequest  ResourceRequest   `json:"resource_request" db:"resource_request"` // JSONB
	ErrorMessage     string            `json:"error_message,omitempty" db:"error_message"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	StartedAt        *time.Time        `json:"started_at,omitempty" db:"started_at"`
	FinishedAt       *time.Time        `json:"finished_at,omitempty" db:"finished_at"`
	UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}

// TrainingConfig 训练配置
type TrainingConfig struct {
	NumTrainEpochs          int     `json:"num_train_epochs"`
	PerDeviceTrainBatchSize int     `json:"per_device_train_batch_size"`
	GradientAccumulationSteps int   `json:"gradient_accumulation_steps,omitempty"`
	LearningRate            float64 `json:"learning_rate"`
	WeightDecay             float64 `json:"weight_decay,omitempty"`
	WarmupRatio             float64 `json:"warmup_ratio,omitempty"`
	LRSchedulerType         string  `json:"lr_scheduler_type,omitempty"` // "cosine", "linear"
	SaveSteps               int     `json:"save_steps,omitempty"`
	LoggingSteps            int     `json:"logging_steps,omitempty"`
	MaxLength               int     `json:"max_length,omitempty"`
	LoRAConfig              *LoRAConfig `json:"lora_config,omitempty"`
}

// LoRAConfig LoRA 微调配置
type LoRAConfig struct {
	Enabled    bool    `json:"enabled"`
	R          int     `json:"r,omitempty"`           // LoRA rank
	Alpha      int     `json:"alpha,omitempty"`       // LoRA alpha
	Dropout    float64 `json:"dropout,omitempty"`
	TargetModules []string `json:"target_modules,omitempty"`
}

// ResourceRequest 资源请求
type ResourceRequest struct {
	GPUCount     int    `json:"gpu_count"`     // GPU 数量
	GPUDeviceIDs string `json:"gpu_device_ids,omitempty"` // GPU 设备 ID，如 "0,1,2"
	GPUType      string `json:"gpu_type,omitempty"` // GPU 类型
	MemoryGB     int    `json:"memory_gb,omitempty"` // 内存 (GB)
	CPUCores     int    `json:"cpu_cores,omitempty"` // CPU 核心数
}

// StageRun 阶段运行实例
type StageRun struct {
	ID             string            `json:"id" db:"id"`
	PipelineRunID  string            `json:"pipeline_run_id" db:"pipeline_run_id"`
	StageType      StageType         `json:"stage_type" db:"stage_type"`
	StageOrder     int               `json:"stage_order" db:"stage_order"` // 阶段序号 (1-6)
	Status         PipelineStatus    `json:"status" db:"status"`
	ContainerID    string            `json:"container_id,omitempty" db:"container_id"`
	NodeName       string            `json:"node_name,omitempty" db:"node_name"`
	ConfigPath     string            `json:"config_path,omitempty" db:"config_path"` // EasyDistill 配置文件路径
	InputManifest  map[string]string `json:"input_manifest,omitempty" db:"input_manifest"` // JSONB
	OutputManifest map[string]string `json:"output_manifest,omitempty" db:"output_manifest"` // JSONB
	Metrics        map[string]interface{} `json:"metrics,omitempty" db:"metrics"` // JSONB
	LogPath        string            `json:"log_path,omitempty" db:"log_path"`
	RetryCount     int               `json:"retry_count" db:"retry_count"`
	ErrorMessage   string            `json:"error_message,omitempty" db:"error_message"`
	StartedAt      *time.Time        `json:"started_at,omitempty" db:"started_at"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

// ContainerRun 容器运行实例
type ContainerRun struct {
	ID            string            `json:"id" db:"id"`
	StageRunID    string            `json:"stage_run_id" db:"stage_run_id"`
	ContainerName string            `json:"container_name" db:"container_name"`
	Image         string            `json:"image" db:"image"`
	NodeName      string            `json:"node_name" db:"node_name"`
	NodeAddr      string            `json:"node_addr" db:"node_addr"`
	Command       string            `json:"command" db:"command"`
	Args          []string          `json:"args" db:"args"` // JSONB
	Envs          map[string]string `json:"envs,omitempty" db:"envs"` // JSONB
	Mounts        []Mount           `json:"mounts,omitempty" db:"mounts"` // JSONB
	XPUAllocation string            `json:"xpu_allocation,omitempty" db:"xpu_allocation"` // GPU/NPU 分配
	ExitCode      *int              `json:"exit_code,omitempty" db:"exit_code"`
	StartedAt     *time.Time        `json:"started_at,omitempty" db:"started_at"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

// Mount 挂载配置
type Mount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

// EvaluationReport 评估报告
type EvaluationReport struct {
	ID            string                 `json:"id" db:"id"`
	PipelineRunID string                 `json:"pipeline_run_id" db:"pipeline_run_id"`
	StageRunID    string                 `json:"stage_run_id" db:"stage_run_id"`
	Metrics       map[string]float64     `json:"metrics" db:"metrics"` // JSONB
	Details       map[string]interface{} `json:"details,omitempty" db:"details"` // JSONB
	Summary       string                 `json:"summary,omitempty" db:"summary"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
}

// WorkerNode Worker 节点信息
type WorkerNode struct {
	NodeName      string    `json:"node_name" redis:"node_name"`
	NodeAddr      string    `json:"node_addr" redis:"node_addr"`
	Status        string    `json:"status" redis:"status"` // "online", "offline", "busy"
	TotalGPU      int       `json:"total_gpu" redis:"total_gpu"`
	AvailableGPU  int       `json:"available_gpu" redis:"available_gpu"`
	TotalMemoryGB int       `json:"total_memory_gb" redis:"total_memory_gb"`
	TotalCPU      int       `json:"total_cpu" redis:"total_cpu"`
	LastHeartbeat time.Time `json:"last_heartbeat" redis:"last_heartbeat"`
	UpdatedAt     time.Time `json:"updated_at" redis:"updated_at"`
}
