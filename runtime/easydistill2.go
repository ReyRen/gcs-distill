package runtime

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/goccy/go-yaml"
	"github.com/google/uuid"
)

const EasyDistill2Revision = "aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3"
const EasyDistill2Image = "gcs/distill-data:2.0.0-aeb8ca3-x86_64"

//go:embed recipes/*.yaml
var recipes embed.FS

type WorkflowRecipe struct {
	JobType      string   `json:"job_type"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	OutputFormat string   `json:"output_format"`
	InputFields  []string `json:"input_fields"`
	Template     string   `json:"-"`
}

func WorkflowRecipes() []WorkflowRecipe {
	return []WorkflowRecipe{
		{"instruct_distill", "基础指令蒸馏", "instruction", "sft_messages", []string{"instruction"}, "instruct_distill"},
		{"advanced_instruct_distill", "高质量指令蒸馏", "instruction", "sft_messages", []string{"instruction"}, "advanced_instruct_distill"},
		{"augmented_instruct_distill", "指令增强蒸馏", "instruction", "sft_messages", []string{"instruction"}, "augmented_instruct_distill"},
		{"balanced_instruct_distill", "类别均衡蒸馏", "instruction", "sft_messages", []string{"instruction"}, "balanced_instruct_distill"},
		{"cot_distill", "思维链蒸馏", "reasoning", "sft_messages", []string{"problem or instruction"}, "cot_distill"},
		{"advanced_cot_distill", "思维链评分与难度混合", "reasoning", "sft_messages", []string{"problem or instruction"}, "advanced_cot_distill"},
		{"cot_long2short", "思维链压缩", "reasoning", "records", []string{"instruction", "response"}, "cot_long2short"},
		{"cot_short2long", "思维链扩展", "reasoning", "records", []string{"instruction", "response"}, "cot_short2long"},
		{"dpo_data_build", "DPO 偏好数据", "preference", "dpo_pairs", []string{"instruction"}, "dpo_instruct"},
		{"instruct_eval", "指令数据质量评估", "evaluation", "scored_records", []string{"instruction", "output or messages"}, "instruct_eval"},
		{"cot_eval", "思维链质量评估", "evaluation", "scored_records", []string{"instruction", "output or messages"}, "cot_eval"},
		{"agent_distill", "Agent 工具轨迹蒸馏", "agent", "sft_messages", []string{"persona or background"}, "agent_distill"},
		{"search_agent_distill", "搜索 Agent 蒸馏", "agent", "sft_messages", []string{"see native config"}, "search_agent_distill"},
		{"mm_instruct_distill", "多模态指令蒸馏", "multimodal", "sft_messages", []string{"instruction", "images"}, "mm_instruct_distill"},
		{"mm_cot_distill", "多模态思维链蒸馏", "multimodal", "sft_messages", []string{"instruction", "images"}, "mm_cot_distill"},
	}
}

func FindWorkflow(job string) (WorkflowRecipe, bool) {
	for _, recipe := range WorkflowRecipes() {
		if recipe.JobType == job {
			return recipe, true
		}
	}
	return WorkflowRecipe{}, false
}

// WorkflowDefaults exposes editable native parameters without teacher
// credentials or storage paths so the frontend can preserve the full recipe.
func WorkflowDefaults(recipe WorkflowRecipe) (map[string]interface{}, error) {
	raw, err := recipes.ReadFile("recipes/" + recipe.Template + "_pai_token.yaml")
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	for _, key := range []string{"job_type", "backend", "dataset"} {
		delete(cfg, key)
	}
	if stages, ok := cfg["pipeline"].([]interface{}); ok {
		for _, stage := range stages {
			if value, ok := stage.(map[string]interface{}); ok {
				delete(value, "output_path")
			}
		}
	}
	return cfg, nil
}

func NormalizeDistillation(p *types.PipelineRun, project *types.Project) error {
	if p.TrainingConfig.Distillation == nil {
		p.TrainingConfig.Distillation = &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}
	}
	w := p.TrainingConfig.Distillation
	if w.Version == "" {
		w.Version = "2.0"
	}
	if w.Version != "2.0" {
		return fmt.Errorf("仅支持 EasyDistill 2.0")
	}
	if w.JobType == "" {
		w.JobType = "instruct_distill"
	}
	recipe, ok := FindWorkflow(w.JobType)
	if !ok {
		return fmt.Errorf("不支持的蒸馏配方: %s", w.JobType)
	}
	if strings.TrimSpace(w.Name) == "" {
		name := []rune(project.Name + " / " + recipe.Name)
		if len(name) > 255 {
			name = name[:255]
		}
		w.Name = string(name)
	}
	if len([]rune(w.Name)) > 255 {
		return fmt.Errorf("任务名称不能超过255个字符")
	}
	if w.TeacherModel == nil {
		teacher := project.TeacherModelConfig
		w.TeacherModel = &teacher
	}
	teacher := w.TeacherModel
	if teacher.MaxTokens < 0 || teacher.Concurrency < 0 || teacher.TimeoutSeconds < 0 {
		return fmt.Errorf("教师生成长度、并发数和超时时间不能为负数")
	}
	if teacher.ModelName == "" {
		return fmt.Errorf("教师模型名称不能为空")
	}
	if teacher.Endpoint == "" {
		return fmt.Errorf("EasyDistill 2 教师需要推理服务 endpoint；本地模型请先通过模型中心部署为 OpenAI 兼容服务")
	}
	endpoint, err := url.Parse(teacher.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("教师 endpoint 必须是有效的 HTTP 或 HTTPS 服务地址")
	}
	if err := types.NormalizeResourceRequest(&p.ResourceRequest, w.TrainStudent); err != nil {
		return err
	}
	if w.TrainStudent {
		if w.StudentModel == nil {
			return fmt.Errorf("训练学生模型时必须选择本次任务的学生基模")
		}
		if recipe.OutputFormat != "sft_messages" && recipe.OutputFormat != "dpo_pairs" {
			return fmt.Errorf("此配方产出不是训练集，请先构建 SFT 或 DPO 数据")
		}
		if w.StudentModel.ProviderType != types.ProviderLocal || w.StudentModel.ModelPath == "" {
			return fmt.Errorf("训练学生模型时必须选择本地学生基模")
		}
		if strings.TrimSpace(w.TrainerImage) == "" || strings.TrimSpace(w.ModelTemplate) == "" {
			return fmt.Errorf("训练学生模型需要 trainer_image 和 model_template（LLaMA-Factory）")
		}
		if w.FinetuningType == "" {
			w.FinetuningType = "lora"
		}
		if w.FinetuningType != "lora" && w.FinetuningType != "full" {
			return fmt.Errorf("finetuning_type 必须是 lora 或 full")
		}
		if p.TrainingConfig.MaxLength == 0 && (w.JobType == "agent_distill" || strings.HasPrefix(w.JobType, "mm_")) {
			p.TrainingConfig.MaxLength = 4096
		}
	} else {
		w.StudentModel = nil
		w.TrainerImage = ""
		w.ModelTemplate = ""
		w.FinetuningType = ""
	}
	return nil
}

func PipelineStageTypes(p *types.PipelineRun) []types.StageType {
	stages := []types.StageType{types.StageTeacherConfig, types.StageDatasetBuild, types.StageDistillData, types.StageDatasetExport}
	if p.TrainingConfig.Distillation != nil && p.TrainingConfig.Distillation.TrainStudent {
		stages = append(stages, types.StageStudentTrain)
	}
	return stages
}

func (g *ConfigGenerator) GenerateDistill2Config(project *types.Project, p *types.PipelineRun) (map[string]interface{}, error) {
	w := p.TrainingConfig.Distillation
	if w == nil {
		return nil, fmt.Errorf("EasyDistill 2 workflow is required")
	}
	recipe, ok := FindWorkflow(w.JobType)
	if !ok {
		return nil, fmt.Errorf("unknown workflow %q", w.JobType)
	}
	raw, err := recipes.ReadFile("recipes/" + recipe.Template + "_pai_token.yaml")
	if err != nil {
		return nil, err
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	mergeConfig(cfg, w.Parameters)
	root := g.GetRunWorkspace(p.UID, project.ID, p.ID)
	cfg["job_type"] = w.JobType
	teacher := project.TeacherModelConfig
	if w.TeacherModel != nil {
		teacher = *w.TeacherModel
	}
	key := firstString(extraString(teacher, "api_key"), teacher.APISecretRef)
	if key == "" {
		key = "EMPTY"
	} // Local OpenAI-compatible services may not authenticate.
	endpoint := strings.TrimSuffix(strings.TrimRight(teacher.Endpoint, "/"), "/chat/completions")
	backend := map[string]interface{}{"type": "openai", "model_id": teacher.ModelName, "base_url": endpoint, "api_key": key}
	if override, ok := cfg["backend"].(map[string]interface{}); ok {
		// Preserve optional timeout/retry settings, but use the selected teacher.
		for _, name := range []string{"timeout", "max_retries"} {
			if value, exists := override[name]; exists {
				backend[name] = value
			}
		}
	}
	cfg["backend"] = backend
	if search, ok := cfg["search_agent"].(map[string]interface{}); ok {
		if tools, ok := search["tools"].(map[string]interface{}); ok && tools["mode"] != "mock" {
			return nil, fmt.Errorf("离线搜索 Agent 只支持模拟工具模式")
		}
		if roles, ok := search["roles"].(map[string]interface{}); ok {
			for _, v := range roles {
				if role, ok := v.(map[string]interface{}); ok {
					role["model_id"] = teacher.ModelName
				}
			}
		}
	}
	dataset, _ := cfg["dataset"].(map[string]interface{})
	if dataset == nil {
		dataset = map[string]interface{}{}
	}
	dataset["input_path"] = filepath.Join(root, "data", "seed", "input.jsonl")
	dataset["output_path"] = distill2OutputPath(root, w.JobType)
	cfg["dataset"] = dataset
	if generation, ok := cfg["generation"].(map[string]interface{}); ok {
		if w.JobType == "agent_distill" {
			system, _ := generation["system_prompt"].(string)
			generation["system_prompt"] = strings.TrimSpace(system) + "\nReturn only the XML tags requested by the task prompt. Do not use Markdown fences, headings, or prose outside those tags. When tools are requested, put a valid JSON array of tool objects inside <tools>...</tools>."
		}
		if teacher.Temperature > 0 {
			generation["temperature"] = teacher.Temperature
		}
		if teacher.MaxTokens > 0 {
			generation["max_tokens"] = teacher.MaxTokens
		}
		if teacher.Concurrency > 0 {
			generation["max_workers"] = teacher.Concurrency
		}
	}
	if teacher.TimeoutSeconds > 0 {
		backend["timeout"] = teacher.TimeoutSeconds
	}
	if pipeline, ok := cfg["pipeline"].([]interface{}); ok {
		for i, rawStage := range pipeline {
			stage, ok := rawStage.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("pipeline stage %d must be an object", i+1)
			}
			name, _ := stage["stage"].(string)
			if w.JobType == "agent_distill" && strings.HasPrefix(name, "agent_") && name != "agent_trajectory" {
				if generation, ok := cfg["generation"].(map[string]interface{}); ok {
					if config, ok := stage["config"].(map[string]interface{}); ok && config["system_prompt"] == nil {
						config["system_prompt"] = generation["system_prompt"]
					}
				}
			}
			if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
				return nil, fmt.Errorf("invalid pipeline stage name")
			}
			stage["output_path"] = filepath.Join(root, "data", "intermediate", fmt.Sprintf("%02d_%s.jsonl", i+1, name))
		}
	}
	applyTeacherGenerationControls(cfg, teacher.MaxTokens, teacher.Concurrency)
	fixRecipePaths(cfg)
	return cfg, nil
}

func applyTeacherGenerationControls(value any, maxTokens, workers int) {
	switch node := value.(type) {
	case map[string]interface{}:
		for key, child := range node {
			switch {
			case key == "max_tokens" && maxTokens > 0:
				node[key] = maxTokens
			case key == "max_workers" && workers > 0:
				node[key] = workers
			default:
				applyTeacherGenerationControls(child, maxTokens, workers)
			}
		}
	case []interface{}:
		for _, child := range node {
			applyTeacherGenerationControls(child, maxTokens, workers)
		}
	}
}

func mergeConfig(dst, src map[string]interface{}) {
	for key, value := range src {
		if child, ok := value.(map[string]interface{}); ok {
			current, _ := dst[key].(map[string]interface{})
			if current == nil {
				current = map[string]interface{}{}
				dst[key] = current
			}
			mergeConfig(current, child)
		} else {
			dst[key] = value
		}
	}
}

func fixRecipePaths(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if text, ok := child.(string); ok && strings.HasSuffix(key, "_file") && strings.HasPrefix(text, "configs/") {
				typed[key] = "/opt/easydistill/" + text
			} else {
				fixRecipePaths(child)
			}
		}
	case []interface{}:
		for _, child := range typed {
			fixRecipePaths(child)
		}
	}
}

func distill2OutputPath(root, job string) string {
	return filepath.Join(root, "data", "exported", "dataset.jsonl")
}

func normalizeImageReferences(rows []map[string]interface{}, base, userRoot string) error {
	normalize := func(raw string) (string, error) {
		if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "data:") {
			return raw, nil
		}
		path := strings.TrimPrefix(raw, "file://")
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("image file is not accessible: %s", raw)
		}
		rel, err := filepath.Rel(userRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return "", fmt.Errorf("image must belong to the current user workspace")
		}
		return resolved, nil
	}
	var walk func(interface{}) error
	walk = func(value interface{}) error {
		switch v := value.(type) {
		case map[string]interface{}:
			for key, child := range v {
				if key == "images" {
					if list, ok := child.([]interface{}); ok {
						for i, item := range list {
							if raw, ok := item.(string); ok {
								path, err := normalize(raw)
								if err != nil {
									return err
								}
								list[i] = path
							}
						}
					}
				}
				if key == "image_url" {
					if image, ok := child.(map[string]interface{}); ok {
						if raw, ok := image["url"].(string); ok {
							path, err := normalize(raw)
							if err != nil {
								return err
							}
							image["url"] = path
						}
					}
				}
				if err := walk(child); err != nil {
					return err
				}
			}
		case []interface{}:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, row := range rows {
		if err := walk(row); err != nil {
			return err
		}
	}
	return nil
}

func (e *StageExecutor) prepareDataset2(ctx context.Context, stage *types.StageRun, project *types.Project, p *types.PipelineRun) error {
	root := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	if err := e.prepareRunDirectories(p.UID, root); err != nil {
		return err
	}
	dataset, err := e.datasetRepo.GetByID(ctx, p.DatasetID)
	if err != nil {
		return err
	}
	userRoot := filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID))
	path := filepath.Join(root, "data", "seed", "input.jsonl")
	count, err := CopyRecords(dataset.FilePath, path, func(row map[string]interface{}) error {
		if err := MapSeedRecord(row, p.TrainingConfig.Distillation); err != nil {
			return err
		}
		return normalizeImageReferences([]map[string]interface{}{row}, filepath.Dir(dataset.FilePath), userRoot)
	})
	if err != nil {
		return err
	}
	stage.OutputManifest = map[string]string{"input_path": path, "record_count": fmt.Sprint(count), "format": "jsonl"}
	return nil
}

func (e *StageExecutor) executeDistill2(ctx context.Context, stage *types.StageRun, project *types.Project, p *types.PipelineRun) error {
	cfg, err := e.configGen.GenerateDistill2Config(project, p)
	if err != nil {
		return err
	}
	root := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	configPath := filepath.Join(root, "configs", "distill_data.json")
	if err := writePrivateJSON(configPath, cfg); err != nil {
		return err
	}
	stage.ConfigPath = configPath
	workers := p.ResourceRequest.ExecutorNodes
	if len(workers) == 0 {
		workers = e.dataWorkers
	}
	containerID, err := e.runContainerTask(ctx, &ContainerRequest{
		ContainerName: stageContainerName(p.ID, stage.StageType), TaskUID: p.UID,
		Image: e.runtimeImage, Command: "gcs-easydistill", Args: []string{"--config", configPath},
		HostWorkDir: root, ConfigPath: configPath, LogPath: e.configGen.GetLogPath(p.UID, project.ID, p.ID, "distill_data"),
		ResourceMode: "cpu",
		WorkersName:  workers,
		Mounts:       []gcsclient.ContainerTaskMount{{Path: filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID)), ReadOnly: true}},
	})
	if err != nil {
		return err
	}
	if err := e.recordContainer(ctx, stage, containerID, e.configGen.GetLogPath(p.UID, project.ID, p.ID, "distill_data")); err != nil {
		return err
	}
	defer e.ReleaseContainer(stage)
	if err := e.waitForContainerTask(ctx, containerID, p.ID, stage); err != nil {
		return err
	}
	stage.OutputManifest = map[string]string{"container_id": containerID, "output_path": distill2OutputPath(root, p.TrainingConfig.Distillation.JobType), "upstream_revision": EasyDistill2Revision}
	return nil
}

func writePrivateJSON(path string, value interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func (e *StageExecutor) executeExport2(ctx context.Context, stage *types.StageRun, project *types.Project, p *types.PipelineRun) error {
	root := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	w := p.TrainingConfig.Distillation
	path := distill2OutputPath(root, w.JobType)
	recipe, _ := FindWorkflow(w.JobType)
	assetID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("gcs-distill2:%d:%s", p.UID, p.ID))).String()
	assetDir := filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID), "train-center", "model-distill", "datasets", "generated", assetID)
	teacher := project.TeacherModelConfig
	if w.TeacherModel != nil {
		teacher = *w.TeacherModel
	}
	if teacher.ModelName == "" {
		return fmt.Errorf("teacher model identity is required for export provenance")
	}
	if strings.HasPrefix(w.JobType, "mm_") {
		if _, err := CopyRecords(path, filepath.Join(root, "data", "intermediate", "mm_native_sft.jsonl"), nil); err != nil {
			return err
		}
	}
	count, err := CopyRecords(path, path, func(row map[string]interface{}) error {
		if recipe.OutputFormat == "sft_messages" {
			if messages, ok := row["messages"].([]interface{}); !ok || len(messages) == 0 {
				return fmt.Errorf("SFT output has no messages")
			}
		}
		if recipe.OutputFormat == "dpo_pairs" && (row["chosen"] == nil || row["rejected"] == nil) {
			return fmt.Errorf("DPO output has no chosen/rejected")
		}
		if strings.HasPrefix(w.JobType, "mm_") {
			if err := normalizeMultimodalRecord(row, filepath.Join(assetDir, "images"), filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID))); err != nil {
				return err
			}
		}
		metadata, ok := row["metadata"].(map[string]interface{})
		if !ok {
			metadata = map[string]interface{}{}
		}
		metadata["model"] = teacher.ModelName
		provenance := map[string]string{"pipeline_id": p.ID, "seed_dataset_id": p.DatasetID, "job_type": w.JobType}
		if teacher.ServiceID != "" {
			provenance["teacher_service_id"] = teacher.ServiceID
		}
		metadata["gcs_distill"] = provenance
		row["metadata"] = metadata
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate distillation output: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	stage.OutputManifest = map[string]string{"dataset_path": path, "format": recipe.OutputFormat, "record_count": fmt.Sprint(count), "bytes": fmt.Sprint(info.Size()), "upstream_revision": EasyDistill2Revision}
	stage.Metrics = map[string]interface{}{"output_records": count, "output_bytes": info.Size()}
	if e.datasetRepo != nil {
		id := assetID
		assetPath := filepath.Join(assetDir, "dataset.jsonl")
		existing, err := e.datasetRepo.GetByID(ctx, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check generated dataset: %w", err)
		}
		if existing == nil || errors.Is(err, sql.ErrNoRows) {
			if _, err := CopyRecords(path, assetPath, nil); err != nil {
				return err
			}
			name := []rune(project.Name + " / " + recipe.Name)
			if len(name) > 200 {
				name = name[:200]
			}
			asset := &types.Dataset{ID: id, UID: p.UID, Name: string(name), SourceType: "generated", FilePath: assetPath, RecordCount: count,
				Description: fmt.Sprintf("EasyDistill 2; pipeline=%s; seed_dataset=%s; job_type=%s; revision=%s", p.ID, p.DatasetID, w.JobType, EasyDistill2Revision)}
			if err := e.datasetRepo.Create(ctx, asset); err != nil {
				return fmt.Errorf("register generated dataset: %w", err)
			}
		} else if existing.UID != p.UID || existing.FilePath != assetPath {
			return fmt.Errorf("generated dataset ownership mismatch")
		}
		stage.OutputManifest["dataset_id"] = id
		stage.OutputManifest["asset_path"] = assetPath
	}
	if err := writePrivateJSON(filepath.Join(root, "data", "exported", "manifest.json"), stage.OutputManifest); err != nil {
		return err
	}
	return nil
}

// BuildTrainerConfig hands the dataset to the existing LLaMA-Factory runtime.
// It does not import or recreate the removed EasyDistill 1 training implementation.
func (g *ConfigGenerator) BuildTrainerConfig(project *types.Project, p *types.PipelineRun) (map[string]interface{}, map[string]interface{}, error) {
	w := p.TrainingConfig.Distillation
	if w == nil || !w.TrainStudent {
		return nil, nil, fmt.Errorf("student training was not requested")
	}
	root := g.GetRunWorkspace(p.UID, project.ID, p.ID)
	train := withTrainingDefaults(p.TrainingConfig)
	stage := "sft"
	dataset := map[string]interface{}{"file_name": distill2OutputPath(root, w.JobType), "formatting": "sharegpt", "columns": map[string]string{"messages": "messages"}, "tags": map[string]string{"role_tag": "role", "content_tag": "content", "user_tag": "user", "assistant_tag": "assistant", "system_tag": "system"}}
	if w.JobType == "dpo_data_build" {
		stage = "dpo"
		dataset = map[string]interface{}{"file_name": distill2OutputPath(root, w.JobType), "ranking": true, "columns": map[string]string{"prompt": "instruction", "query": "input", "chosen": "chosen", "rejected": "rejected"}}
	}
	if strings.HasPrefix(w.JobType, "mm_") {
		dataset["columns"].(map[string]string)["images"] = "images"
	}
	cfg := map[string]interface{}{"model_name_or_path": w.StudentModel.ModelPath, "trust_remote_code": true, "stage": stage, "do_train": true,
		"finetuning_type": w.FinetuningType, "dataset": "gcs_distilled", "dataset_dir": filepath.Join(root, "configs"), "template": w.ModelTemplate,
		"output_dir": filepath.Join(root, "models", "checkpoints"), "logging_dir": filepath.Join(root, "logs", "student_train", "trainer"), "overwrite_output_dir": false, "report_to": "none", "plot_loss": true,
		"num_train_epochs": train.NumTrainEpochs, "per_device_train_batch_size": train.PerDeviceTrainBatchSize, "gradient_accumulation_steps": train.GradientAccumulationSteps,
		"learning_rate": train.LearningRate, "cutoff_len": train.MaxLength, "logging_steps": train.LoggingSteps, "save_steps": train.SaveSteps, "fp16": true, "flash_attn": "sdpa", "save_safetensors": true}
	cfg["weight_decay"] = train.WeightDecay
	cfg["warmup_ratio"] = train.WarmupRatio
	if train.LRSchedulerType != "" {
		cfg["lr_scheduler_type"] = train.LRSchedulerType
	}
	if w.FinetuningType == "lora" {
		cfg["lora_target"] = "all"
	}
	return cfg, map[string]interface{}{"gcs_distilled": dataset}, nil
}

func (e *StageExecutor) executeStudentTrain2(ctx context.Context, stage *types.StageRun, project *types.Project, p *types.PipelineRun) error {
	w := p.TrainingConfig.Distillation
	if w == nil || w.StudentModel == nil || w.StudentModel.ModelID == "" {
		return fmt.Errorf("student model is required")
	}
	storage := config.StorageConfig{RootPath: e.configGen.storageRoot}
	base, err := storage.UserStudentModels(p.UID)
	if err != nil {
		return err
	}
	modelPath, err := SafeArtifactPath(base, w.StudentModel.ModelID)
	if err != nil || modelPath != w.StudentModel.ModelPath {
		return fmt.Errorf("student model must belong to the current user's private catalog")
	}
	cfg, datasets, err := e.configGen.BuildTrainerConfig(project, p)
	if err != nil {
		return err
	}
	root := e.configGen.GetRunWorkspace(p.UID, project.ID, p.ID)
	path := filepath.Join(root, "configs", "student_train.json")
	if err := writePrivateJSON(path, cfg); err != nil {
		return err
	}
	if err := writePrivateJSON(filepath.Join(root, "configs", "dataset_info.json"), datasets); err != nil {
		return err
	}
	stage.ConfigPath = path
	id, err := e.runContainerTask(ctx, &ContainerRequest{ContainerName: stageContainerName(p.ID, stage.StageType), TaskUID: p.UID,
		Image: p.TrainingConfig.Distillation.TrainerImage, Command: "llamafactory-cli", Args: []string{"train", path},
		HostWorkDir: root, ConfigPath: path, LogPath: e.configGen.GetLogPath(p.UID, project.ID, p.ID, "student_train"),
		GPUs: p.ResourceRequest.GPUCount, SelectedResources: p.ResourceRequest.SelectedResources,
		Mounts: []gcsclient.ContainerTaskMount{{Path: w.StudentModel.ModelPath, ReadOnly: true}, {Path: filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", p.UID)), ReadOnly: true}}})
	if err != nil {
		return err
	}
	if err := e.recordContainer(ctx, stage, id, e.configGen.GetLogPath(p.UID, project.ID, p.ID, "student_train")); err != nil {
		return err
	}
	defer e.ReleaseContainer(stage)
	if err := e.waitForContainerTask(ctx, id, p.ID, stage); err != nil {
		return err
	}
	checkpoints := filepath.Join(root, "models", "checkpoints")
	entries, err := os.ReadDir(checkpoints)
	weights := false
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (strings.HasSuffix(name, ".safetensors") || strings.HasSuffix(name, ".bin")) && (strings.HasPrefix(name, "adapter_model") || strings.HasPrefix(name, "model") || strings.HasPrefix(name, "pytorch_model")) {
			if info, statErr := entry.Info(); statErr == nil && info.Size() > 0 {
				weights = true
			}
		}
	}
	if err != nil || !weights {
		return fmt.Errorf("training finished without checkpoint artifacts")
	}
	stage.OutputManifest = map[string]string{"container_id": id, "checkpoint_path": checkpoints, "trainer": "llamafactory"}
	return nil
}
