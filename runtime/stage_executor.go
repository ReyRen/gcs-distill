package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/logger"
	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
	"go.uber.org/zap"
)

type StageExecutor struct {
	configGen    *ConfigGenerator
	manifestMgr  *ManifestManager
	dataGovernor *DataGovernor
	datasetRepo  mysqlrepo.DatasetRepository
	gcsClient    *gcsclient.Client
	runtimeImage string
}

func NewStageExecutor(workspaceRoot string, datasetRepo mysqlrepo.DatasetRepository, gcsClient *gcsclient.Client, runtimeImage string) *StageExecutor {
	if strings.TrimSpace(runtimeImage) == "" {
		runtimeImage = "easy-distill/easydistill:latest"
	}
	return &StageExecutor{
		configGen:    NewConfigGenerator(workspaceRoot),
		manifestMgr:  NewManifestManager(workspaceRoot),
		dataGovernor: NewDataGovernor(),
		datasetRepo:  datasetRepo,
		gcsClient:    gcsClient,
		runtimeImage: runtimeImage,
	}
}

func (e *StageExecutor) ExecuteStage(ctx context.Context, stage *types.StageRun, pipeline *types.PipelineRun, project *types.Project) error {
	logger.Info("execute distill stage",
		zap.String("stage_type", string(stage.StageType)),
		zap.String("stage_id", stage.ID),
	)

	switch stage.StageType {
	case types.StageTeacherConfig:
		return e.executeTeacherConfig(stage, project)
	case types.StageDatasetBuild:
		return e.executeDatasetBuild(ctx, stage, project, pipeline)
	case types.StageTeacherInfer:
		return e.executeTeacherInfer(ctx, stage, project, pipeline)
	case types.StageDataGovern:
		return e.executeDataGovern(stage, project, pipeline)
	case types.StageStudentTrain:
		return e.executeStudentTrain(ctx, stage, project, pipeline)
	case types.StageEvaluate:
		return e.executeEvaluate(ctx, stage, project, pipeline)
	default:
		return fmt.Errorf("unknown stage type: %s", stage.StageType)
	}
}

func (e *StageExecutor) executeTeacherConfig(stage *types.StageRun, project *types.Project) error {
	config := project.TeacherModelConfig
	if strings.TrimSpace(config.ModelName) == "" {
		return fmt.Errorf("teacher model name is required")
	}

	switch config.ProviderType {
	case types.ProviderAPI:
		if strings.TrimSpace(config.Endpoint) == "" {
			return fmt.Errorf("api teacher endpoint is required")
		}
		if strings.TrimSpace(config.APISecretRef) == "" && extraString(config, "api_key") == "" {
			return fmt.Errorf("api teacher api_secret_ref or extra_params.api_key is required")
		}
	case types.ProviderLocal:
		if localModelPath(config) == "" {
			return fmt.Errorf("local teacher model_path is required")
		}
	default:
		return fmt.Errorf("unsupported teacher provider_type: %s", config.ProviderType)
	}

	stage.OutputManifest = map[string]string{
		"teacher_model": config.ModelName,
		"provider_type": string(config.ProviderType),
		"validated_at":  time.Now().Format(time.RFC3339),
	}
	return nil
}

func (e *StageExecutor) executeDatasetBuild(ctx context.Context, stage *types.StageRun, project *types.Project, pipeline *types.PipelineRun) error {
	projectID := project.ID
	runID := pipeline.ID
	workspace := e.configGen.GetRunWorkspace(projectID, runID)

	dirs := []string{
		filepath.Join(workspace, "configs"),
		filepath.Join(workspace, "data", "seed"),
		filepath.Join(workspace, "data", "generated"),
		filepath.Join(workspace, "data", "filtered"),
		filepath.Join(workspace, "logs", "teacher_infer"),
		filepath.Join(workspace, "logs", "student_train"),
		filepath.Join(workspace, "logs", "evaluate"),
		filepath.Join(workspace, "models", "checkpoints"),
		filepath.Join(workspace, "eval"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create workspace directory %s: %w", dir, err)
		}
	}

	instructions, err := e.loadDatasetInstructions(ctx, pipeline.DatasetID)
	if err != nil {
		return err
	}
	if err := e.manifestMgr.CreateSeedManifest(projectID, runID, instructions); err != nil {
		return fmt.Errorf("create seed manifest: %w", err)
	}
	templatePath, err := e.manifestMgr.CreateDefaultChatTemplate(projectID, runID)
	if err != nil {
		return err
	}

	stage.OutputManifest = map[string]string{
		"seed_count":    fmt.Sprintf("%d", len(instructions)),
		"workspace":     workspace,
		"template_path": templatePath,
		"created_at":    time.Now().Format(time.RFC3339),
	}
	return nil
}

func (e *StageExecutor) executeTeacherInfer(ctx context.Context, stage *types.StageRun, project *types.Project, pipeline *types.PipelineRun) error {
	projectID := project.ID
	runID := pipeline.ID

	configData, err := e.configGen.GenerateTeacherInferConfig(project, runID)
	if err != nil {
		return fmt.Errorf("generate teacher infer config: %w", err)
	}
	configPath := e.configGen.GetConfigPath(projectID, runID, "teacher_infer")
	if err := writeConfig(configPath, configData); err != nil {
		return err
	}
	stage.ConfigPath = configPath

	containerID, err := e.runContainerTask(ctx, &ContainerRequest{
		ContainerName:     stageContainerName(pipeline.ID, stage.StageType),
		Image:             e.runtimeImage,
		Command:           "python",
		Args:              []string{"-m", "easydistill.kd.infer", "--config", configPath},
		HostWorkDir:       e.configGen.GetRunWorkspace(projectID, runID),
		ConfigPath:        configPath,
		LogPath:           e.configGen.GetLogPath(projectID, runID, "teacher_infer"),
		Env:               "GCS_DISTILL_STAGE=teacher_infer;GCS_DISTILL_PIPELINE_ID=" + pipeline.ID,
		GPUs:              pipeline.ResourceRequest.GPUCount,
		GPUDeviceIDs:      pipeline.ResourceRequest.GPUDeviceIDs,
		SelectedResources: pipeline.ResourceRequest.SelectedResources,
	})
	if err != nil {
		return fmt.Errorf("start teacher infer container: %w", err)
	}
	if err := e.waitForContainerTask(ctx, containerID); err != nil {
		return fmt.Errorf("teacher infer container failed: %w", err)
	}

	stats, _ := e.manifestMgr.GetManifestStats(projectID, runID)
	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "teacher_infer")
	stage.OutputManifest = map[string]string{
		"container_id":  containerID,
		"labeled_count": fmt.Sprintf("%d", stats["labeled"]),
		"config_path":   configPath,
	}
	return nil
}

func (e *StageExecutor) executeDataGovern(stage *types.StageRun, project *types.Project, pipeline *types.PipelineRun) error {
	projectID := project.ID
	runID := pipeline.ID

	labeled, err := e.manifestMgr.LoadLabeledData(projectID, runID)
	if err != nil {
		return fmt.Errorf("load labeled data: %w", err)
	}

	train, test, stats := e.dataGovernor.FilterData(labeled)
	if err := e.manifestMgr.SaveFilteredData(projectID, runID, train, test); err != nil {
		return fmt.Errorf("save filtered data: %w", err)
	}

	filterRate := 0.0
	if stats["total"] > 0 {
		filterRate = float64(stats["filtered"]) / float64(stats["total"])
	}
	stage.OutputManifest = map[string]string{
		"train_count": fmt.Sprintf("%d", len(train)),
		"test_count":  fmt.Sprintf("%d", len(test)),
		"filter_rate": fmt.Sprintf("%.4f", filterRate),
	}
	stage.Metrics = map[string]interface{}{
		"stats":       stats,
		"train_count": len(train),
		"test_count":  len(test),
		"filter_rate": filterRate,
	}
	return nil
}

func (e *StageExecutor) executeStudentTrain(ctx context.Context, stage *types.StageRun, project *types.Project, pipeline *types.PipelineRun) error {
	projectID := project.ID
	runID := pipeline.ID

	configData, err := e.configGen.GenerateStudentTrainConfig(project, pipeline, runID)
	if err != nil {
		return fmt.Errorf("generate student train config: %w", err)
	}
	configPath := e.configGen.GetConfigPath(projectID, runID, "student_train")
	if err := writeConfig(configPath, configData); err != nil {
		return err
	}
	stage.ConfigPath = configPath

	xpuCount := xpuCountForRequest(pipeline.ResourceRequest.GPUCount, pipeline.ResourceRequest.GPUDeviceIDs)
	containerID, err := e.runContainerTask(ctx, &ContainerRequest{
		ContainerName:     stageContainerName(pipeline.ID, stage.StageType),
		Image:             e.runtimeImage,
		Command:           "accelerate",
		Args:              easyDistillTrainArgs(configPath, xpuCount),
		HostWorkDir:       e.configGen.GetRunWorkspace(projectID, runID),
		ConfigPath:        configPath,
		LogPath:           e.configGen.GetLogPath(projectID, runID, "student_train"),
		Env:               "GCS_DISTILL_STAGE=student_train;GCS_DISTILL_PIPELINE_ID=" + pipeline.ID,
		GPUs:              pipeline.ResourceRequest.GPUCount,
		GPUDeviceIDs:      pipeline.ResourceRequest.GPUDeviceIDs,
		SelectedResources: pipeline.ResourceRequest.SelectedResources,
	})
	if err != nil {
		return fmt.Errorf("start student train container: %w", err)
	}
	if err := e.waitForContainerTask(ctx, containerID); err != nil {
		return fmt.Errorf("student train container failed: %w", err)
	}

	checkpointPath := filepath.Join(e.configGen.GetRunWorkspace(projectID, runID), "models", "checkpoints")
	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "student_train")
	stage.OutputManifest = map[string]string{
		"container_id":    containerID,
		"checkpoint_path": checkpointPath,
		"config_path":     configPath,
	}
	return nil
}

func (e *StageExecutor) executeEvaluate(ctx context.Context, stage *types.StageRun, project *types.Project, pipeline *types.PipelineRun) error {
	projectID := project.ID
	runID := pipeline.ID

	configData, err := e.configGen.GenerateEvaluateConfig(project, runID)
	if err != nil {
		return fmt.Errorf("generate evaluate config: %w", err)
	}
	configPath := e.configGen.GetConfigPath(projectID, runID, "evaluate")
	if err := writeConfig(configPath, configData); err != nil {
		return err
	}
	stage.ConfigPath = configPath

	containerID, err := e.runContainerTask(ctx, &ContainerRequest{
		ContainerName:     stageContainerName(pipeline.ID, stage.StageType),
		Image:             e.runtimeImage,
		Command:           "python",
		Args:              []string{"-m", "easydistill.eval.data_eval", "--config", configPath},
		HostWorkDir:       e.configGen.GetRunWorkspace(projectID, runID),
		ConfigPath:        configPath,
		LogPath:           e.configGen.GetLogPath(projectID, runID, "evaluate"),
		Env:               "GCS_DISTILL_STAGE=evaluate;GCS_DISTILL_PIPELINE_ID=" + pipeline.ID,
		GPUs:              1,
		GPUDeviceIDs:      pipeline.ResourceRequest.GPUDeviceIDs,
		SelectedResources: pipeline.ResourceRequest.SelectedResources,
	})
	if err != nil {
		return fmt.Errorf("start evaluate container: %w", err)
	}
	if err := e.waitForContainerTask(ctx, containerID); err != nil {
		return fmt.Errorf("evaluate container failed: %w", err)
	}

	resultPath := filepath.Join(e.configGen.GetRunWorkspace(projectID, runID), "eval", "results.json")
	metrics, err := e.parseEvaluationResults(resultPath)
	if err != nil {
		logger.Warn("parse evaluation results failed", zap.String("result_path", resultPath), zap.Error(err))
		metrics = map[string]interface{}{"error": err.Error()}
	}

	stage.ContainerID = containerID
	stage.LogPath = e.configGen.GetLogPath(projectID, runID, "evaluate")
	stage.Metrics = metrics
	stage.OutputManifest = map[string]string{
		"container_id": containerID,
		"result_path":  resultPath,
		"config_path":  configPath,
	}
	return nil
}

func (e *StageExecutor) loadDatasetInstructions(ctx context.Context, datasetID string) ([]Instruction, error) {
	dataset, err := e.datasetRepo.GetByID(ctx, datasetID)
	if err != nil {
		return nil, fmt.Errorf("get dataset: %w", err)
	}
	if strings.TrimSpace(dataset.FilePath) == "" {
		return nil, fmt.Errorf("dataset file_path is required")
	}

	data, err := os.ReadFile(dataset.FilePath)
	if err != nil {
		return nil, fmt.Errorf("read dataset file: %w", err)
	}

	instructions, scanned, err := parseInstructionFile(data)
	if err != nil {
		return nil, err
	}
	if len(instructions) == 0 {
		return nil, fmt.Errorf("dataset contains no valid instruction records")
	}

	logger.Info("dataset loaded",
		zap.String("dataset_id", datasetID),
		zap.String("file_path", dataset.FilePath),
		zap.Int("scanned_records", scanned),
		zap.Int("valid_instructions", len(instructions)),
	)
	return instructions, nil
}

func parseInstructionFile(data []byte) ([]Instruction, int, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, 0, nil
	}

	if trimmed[0] == '[' {
		var items []Instruction
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, 0, fmt.Errorf("parse dataset JSON array: %w", err)
		}
		return validInstructions(items), len(items), nil
	}

	var items []Instruction
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item Instruction
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			logger.Warn("skip invalid dataset JSONL line", zap.Int("line", lineNum), zap.Error(err))
			continue
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, lineNum, fmt.Errorf("read dataset JSONL: %w", err)
	}
	return validInstructions(items), lineNum, nil
}

func validInstructions(items []Instruction) []Instruction {
	out := make([]Instruction, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Instruction) != "" {
			out = append(out, item)
		}
	}
	return out
}

func writeConfig(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func (e *StageExecutor) parseEvaluationResults(resultPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("read evaluation results: %w", err)
	}
	var results map[string]interface{}
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parse evaluation results JSON: %w", err)
	}
	return results, nil
}

func localModelPath(config types.ModelConfig) string {
	if strings.TrimSpace(config.ModelPath) != "" {
		return strings.TrimSpace(config.ModelPath)
	}
	return strings.TrimSpace(extraString(config, "model_path"))
}

func stageContainerName(pipelineID string, stageType types.StageType) string {
	raw := "distill-" + pipelineID + "-" + string(stageType)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		valid := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if valid {
			b.WriteRune(r)
			lastDash = r == '-'
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "distill-stage"
	}
	if len(out) > 120 {
		return out[:120]
	}
	return out
}

func stableTaskID(seed string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	value := int(h.Sum32() % 1000000000)
	if value <= 0 {
		return int(time.Now().UnixNano() % 1000000000)
	}
	return value
}

func selectedResourcesForGCS(resources []types.SelectedResource) []gcsclient.SelectedResource {
	out := make([]gcsclient.SelectedResource, 0, len(resources))
	for _, item := range resources {
		out = append(out, gcsclient.SelectedResource{
			NodeName:    item.NodeName,
			NodeAddress: item.NodeAddress,
			XPUIndices:  append([]int(nil), item.XPUIndices...),
		})
	}
	return out
}

func xpuCountForRequest(gpuCount int, gpuDeviceIDs string) int {
	if gpuCount > 0 {
		return gpuCount
	}
	ids := strings.Split(gpuDeviceIDs, ",")
	count := 0
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			count++
		}
	}
	if count > 0 {
		return count
	}
	return 1
}

type ContainerRequest struct {
	ContainerName     string
	Image             string
	Command           string
	Args              []string
	HostWorkDir       string
	ConfigPath        string
	LogPath           string
	Env               string
	GPUs              int
	GPUDeviceIDs      string
	SelectedResources []types.SelectedResource
}

func (e *StageExecutor) runContainerTask(ctx context.Context, req *ContainerRequest) (string, error) {
	if e.gcsClient == nil {
		return "", fmt.Errorf("gcs client is not configured")
	}
	workDir := strings.TrimSpace(req.HostWorkDir)
	if workDir == "" {
		return "", fmt.Errorf("container working_dir is required")
	}
	containerName := strings.TrimSpace(req.ContainerName)
	if containerName == "" {
		return "", fmt.Errorf("container name is required")
	}
	configPath := strings.TrimSpace(req.ConfigPath)
	if configPath == "" {
		return "", fmt.Errorf("config path is required")
	}

	args := append([]string(nil), req.Args...)
	if len(args) == 0 {
		args = []string{"--config", configPath}
	}

	resp, err := e.gcsClient.CreateContainerTask(ctx, gcsclient.ContainerTaskRequest{
		TaskUID:           0,
		TaskID:            stableTaskID(containerName + "|" + workDir),
		ContainerName:     containerName,
		Image:             req.Image,
		Command:           req.Command,
		Args:              args,
		WorkingDir:        workDir,
		LogPath:           req.LogPath,
		Envs:              req.Env,
		WorkerNums:        1,
		XPUNums:           xpuCountForRequest(req.GPUs, req.GPUDeviceIDs),
		SelectedResources: selectedResourcesForGCS(req.SelectedResources),
	})
	if err != nil {
		return "", err
	}
	if resp.ContainerName != "" {
		return resp.ContainerName, nil
	}
	return containerName, nil
}

func easyDistillTrainArgs(configPath string, xpuCount int) []string {
	args := []string{"launch"}
	if xpuCount > 1 {
		args = append(args, "--multi_gpu")
	}
	args = append(args,
		"--num_processes", strconv.Itoa(maxInt(1, xpuCount)),
		"--module", "easydistill.kd.train",
		"--config", configPath,
	)
	return args
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (e *StageExecutor) waitForContainerTask(ctx context.Context, containerName string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait gcs-v2 container task timeout: %w", ctx.Err())
		case <-ticker.C:
			task, found, err := e.gcsClient.GetTask(ctx, containerName)
			if err != nil {
				logger.Warn("query gcs-v2 task failed", zap.String("container_name", containerName), zap.Error(err))
				continue
			}
			if !found {
				logger.Warn("gcs-v2 task not found yet", zap.String("container_name", containerName))
				continue
			}
			if task.TaskStates == gcsclient.TaskStateContainerDone {
				return nil
			}
			if task.TaskStates >= gcsclient.TaskStateBaseError {
				return fmt.Errorf("gcs-v2 container task failed, state=%d", task.TaskStates)
			}
		}
	}
}

func (e *StageExecutor) ReadLogFile(projectID, runID, stageName string) (string, error) {
	logPath := e.configGen.GetLogPath(projectID, runID, stageName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read log file: %w", err)
	}
	return string(content), nil
}

func (e *StageExecutor) TailLogFile(projectID, runID, stageName string, lines int) (string, error) {
	logPath := e.configGen.GetLogPath(projectID, runID, stageName)
	content, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("read log file: %w", err)
	}
	allLines := strings.Split(string(content), "\n")
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}
	return strings.Join(allLines[start:], "\n"), nil
}
