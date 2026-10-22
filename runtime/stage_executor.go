package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
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
	datasetRepo  mysqlrepo.DatasetRepository
	stageRepo    mysqlrepo.StageRepository
	pipelineRepo mysqlrepo.PipelineRepository
	gcsClient    *gcsclient.Client
	runtimeImage string
	dataWorkers  []string
}

type StageExecutorOptions struct{ DataWorkers []string }

func NewStageExecutor(storageRoot string, datasetRepo mysqlrepo.DatasetRepository, gcsClient *gcsclient.Client, runtimeImage string, stageRepo mysqlrepo.StageRepository, pipelineRepo mysqlrepo.PipelineRepository, options ...StageExecutorOptions) *StageExecutor {
	if strings.TrimSpace(runtimeImage) == "" {
		runtimeImage = EasyDistill2Image
	}
	var workers []string
	if len(options) > 0 {
		workers = append(workers, options[0].DataWorkers...)
	}
	return &StageExecutor{configGen: NewConfigGenerator(storageRoot), datasetRepo: datasetRepo, stageRepo: stageRepo, pipelineRepo: pipelineRepo, gcsClient: gcsClient, runtimeImage: runtimeImage, dataWorkers: workers}
}

func (e *StageExecutor) ExecuteStage(ctx context.Context, stage *types.StageRun, pipeline *types.PipelineRun, project *types.Project) (resultErr error) {
	if pipeline.UID <= 0 || pipeline.UID != project.UID || pipeline.ProjectID != project.ID {
		return fmt.Errorf("stage workspace ownership mismatch")
	}
	root := e.configGen.GetRunWorkspace(pipeline.UID, project.ID, pipeline.ID)
	if err := e.prepareRunDirectories(pipeline.UID, root); err != nil {
		return err
	}
	stage.LogPath = filepath.Join(root, "logs", string(stage.StageType))
	if filepath.Base(string(stage.StageType)) != string(stage.StageType) || strings.Contains(string(stage.StageType), "..") {
		return fmt.Errorf("invalid stage log directory")
	}
	relativeLogPath, err := filepath.Rel(e.configGen.storageRoot, stage.LogPath)
	if err != nil {
		return err
	}
	if _, err := EnsureStorageDir(e.configGen.storageRoot, relativeLogPath); err != nil {
		return err
	}
	if err := appendStageEvent(stage, "started", nil); err != nil {
		return err
	}
	defer func() {
		status := "succeeded"
		if resultErr != nil {
			status = "failed"
			if resultErr == context.Canceled {
				status = "canceled"
			}
		}
		if err := appendStageEvent(stage, status, resultErr); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("persist stage log: %w", err)
		}
	}()
	w := pipeline.TrainingConfig.Distillation
	if w == nil {
		return fmt.Errorf("retired workflow: create an EasyDistill 2 task")
	}
	snapshot := *project
	if w.TeacherModel != nil {
		snapshot.TeacherModelConfig = *w.TeacherModel
	}
	project = &snapshot
	switch stage.StageType {
	case types.StageTeacherConfig:
		if project.TeacherModelConfig.Endpoint == "" || project.TeacherModelConfig.ModelName == "" {
			return fmt.Errorf("teacher endpoint and model_name are required")
		}
		stage.OutputManifest = map[string]string{"engine": "easydistill-2.0", "teacher_model": project.TeacherModelConfig.ModelName, "upstream_revision": EasyDistill2Revision}
		return nil
	case types.StageDatasetBuild:
		return e.prepareDataset2(ctx, stage, project, pipeline)
	case types.StageDistillData:
		return e.executeDistill2(ctx, stage, project, pipeline)
	case types.StageDatasetExport:
		return e.executeExport2(ctx, stage, project, pipeline)
	case types.StageStudentTrain:
		return e.executeStudentTrain2(ctx, stage, project, pipeline)
	default:
		return fmt.Errorf("unsupported stage: %s", stage.StageType)
	}
}

func appendStageEvent(stage *types.StageRun, status string, stageErr error) error {
	if err := os.MkdirAll(stage.LogPath, 0750); err != nil {
		return err
	}
	event := map[string]interface{}{"time": time.Now().UTC().Format(time.RFC3339Nano), "stage": stage.StageType, "status": status}
	if stageErr != nil {
		event["error"] = stageErr.Error()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(stage.LogPath, "events.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (e *StageExecutor) prepareRunDirectories(uid int, root string) error {
	userRoot := filepath.Join(e.configGen.storageRoot, fmt.Sprintf("user-%d", uid))
	rel, err := filepath.Rel(userRoot, root)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("run workspace must belong to the current user")
	}
	for _, dir := range []string{"configs", "data/seed", "data/intermediate", "data/exported", "models/checkpoints", "logs", "cache/home", "cache/huggingface/datasets", "cache/torch", "cache/torch_extensions", "cache/triton", "cache/matplotlib", "tmp"} {
		path := filepath.Join(root, filepath.FromSlash(dir))
		relative, err := filepath.Rel(e.configGen.storageRoot, path)
		if err != nil {
			return err
		}
		if _, err := EnsureStorageDir(e.configGen.storageRoot, relative); err != nil {
			return err
		}
	}
	return os.Chmod(filepath.Join(root, "configs"), 0700)
}

func containerCacheEnv(root string) string {
	values := []string{"HOME=" + filepath.Join(root, "cache", "home"), "TMPDIR=" + filepath.Join(root, "tmp")}
	for _, item := range [][2]string{{"HF_HOME", "huggingface"}, {"HF_DATASETS_CACHE", "huggingface/datasets"}, {"TORCH_HOME", "torch"}, {"TORCH_EXTENSIONS_DIR", "torch_extensions"}, {"TRITON_CACHE_DIR", "triton"}, {"MPLCONFIGDIR", "matplotlib"}, {"XDG_CACHE_HOME", "."}} {
		values = append(values, item[0]+"="+filepath.Join(root, "cache", filepath.FromSlash(item[1])))
	}
	return strings.Join(values, ";")
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

type ContainerRequest struct {
	ContainerName     string
	TaskUID           int
	Image             string
	Command           string
	Args              []string
	HostWorkDir       string
	ConfigPath        string
	LogPath           string
	Env               string
	GPUs              int
	ResourceMode      string
	WorkersName       []string
	Mounts            []gcsclient.ContainerTaskMount
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
	if req.TaskUID <= 0 {
		return "", fmt.Errorf("container task uid is required")
	}
	if err := e.prepareRunDirectories(req.TaskUID, workDir); err != nil {
		return "", err
	}
	configPath := strings.TrimSpace(req.ConfigPath)
	if configPath == "" {
		return "", fmt.Errorf("config path is required")
	}

	args := append([]string(nil), req.Args...)
	if len(args) == 0 {
		args = []string{"--config", configPath}
	}

	xpuCount := req.GPUs
	if xpuCount <= 0 {
		xpuCount = 1
	}
	if len(req.SelectedResources) > 0 {
		xpuCount = len(req.SelectedResources[0].XPUIndices)
	}
	if req.ResourceMode == "cpu" {
		xpuCount = 0
	}
	if req.ResourceMode != "cpu" && len(req.SelectedResources) > 0 {
		if err := e.gcsClient.ValidateGPUSelection(ctx, selectedResourcesForGCS(req.SelectedResources), true); err != nil {
			return "", fmt.Errorf("训练资源已变化，请重新选择: %w", err)
		}
	}
	resp, err := e.gcsClient.CreateContainerTask(ctx, gcsclient.ContainerTaskRequest{
		TaskUID:           req.TaskUID,
		TaskID:            stableTaskID(containerName + "|" + workDir),
		ContainerName:     containerName,
		Image:             req.Image,
		ImagePullPolicy:   "never",
		Command:           req.Command,
		Args:              args,
		WorkingDir:        workDir,
		LogPath:           req.LogPath,
		Envs:              "GCS_APPLICATION=gcs-distill;HF_HUB_OFFLINE=1;TRANSFORMERS_OFFLINE=1;HF_DATASETS_OFFLINE=1;HF_HUB_DISABLE_TELEMETRY=1;PIP_NO_INDEX=1;" + req.Env + ";" + containerCacheEnv(workDir),
		WorkerNums:        1,
		WorkersName:       append([]string(nil), req.WorkersName...),
		Mounts:            append([]gcsclient.ContainerTaskMount(nil), req.Mounts...),
		XPUNums:           xpuCount,
		ResourceMode:      req.ResourceMode,
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

func (e *StageExecutor) recordContainer(ctx context.Context, stage *types.StageRun, containerName, logPath string) error {
	stage.ContainerID = containerName
	stage.LogPath = logPath
	if e.stageRepo != nil {
		if err := e.stageRepo.Update(ctx, stage); err != nil {
			e.stopContainerTask(containerName)
			return fmt.Errorf("persist stage container before waiting: %w", err)
		}
	}
	return nil
}

func (e *StageExecutor) stopContainerTask(containerName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.gcsClient.DeleteTask(ctx, containerName); err != nil {
		logger.Warn("stop distill container failed", zap.String("container_name", containerName), zap.Error(err))
	}
}

func (e *StageExecutor) ReleaseContainer(stage *types.StageRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Shared worker logs survive container removal; retain Docker output as well.
	if logs, err := e.gcsClient.GetTaskLogs(ctx, stage.ContainerID, "all"); err == nil {
		_ = os.MkdirAll(stage.LogPath, 0755)
		_ = os.WriteFile(filepath.Join(stage.LogPath, "archived.log"), logs, 0600)
	}
	e.stopContainerTask(stage.ContainerID)
}

func (e *StageExecutor) waitForContainerTask(ctx context.Context, containerName, pipelineID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.stopContainerTask(containerName)
			return fmt.Errorf("wait gcs-v2 container task timeout: %w", ctx.Err())
		case <-ticker.C:
			if e.pipelineRepo != nil {
				pipeline, err := e.pipelineRepo.GetByID(ctx, pipelineID)
				if err != nil || pipeline.Status == types.StatusCanceled {
					e.stopContainerTask(containerName)
					if err != nil {
						return fmt.Errorf("pipeline is no longer available: %w", err)
					}
					return context.Canceled
				}
			}
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
