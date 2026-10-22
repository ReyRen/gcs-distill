package runtime

import (
	"context"
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

func (e *StageExecutor) ExecuteStage(ctx context.Context, stage *types.StageRun, pipeline *types.PipelineRun, project *types.Project) error {
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
		Envs:              "GCS_APPLICATION=gcs-distill;HF_HUB_OFFLINE=1;TRANSFORMERS_OFFLINE=1;HF_DATASETS_OFFLINE=1;HF_HUB_DISABLE_TELEMETRY=1;PIP_NO_INDEX=1;" + req.Env,
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
