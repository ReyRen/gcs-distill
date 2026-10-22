package runtime

import (
	"github.com/ReyRen/gcs-distill/internal/types"
	"path/filepath"
	"strconv"
	"strings"
)

type ConfigGenerator struct{ storageRoot string }

func NewConfigGenerator(root string) *ConfigGenerator {
	root = strings.TrimRight(strings.TrimSpace(root), "/")
	if root == "" {
		root = "/storage-root-jfs"
	}
	return &ConfigGenerator{storageRoot: root}
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

func extraString(config types.ModelConfig, key string) string {
	return stringFromMap(config.ExtraParams, key)
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

func firstString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (g *ConfigGenerator) GetRunWorkspace(uid int, projectID, runID string) string {
	return filepath.Join(g.storageRoot, "user-"+strconv.Itoa(uid), "train-center", "model-distill", "projects", projectID, "runs", runID)
}

func (g *ConfigGenerator) GetLogPath(uid int, projectID, runID, stageName string) string {
	return filepath.Join(g.GetRunWorkspace(uid, projectID, runID), "logs", stageName)
}
