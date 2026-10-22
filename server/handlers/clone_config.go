package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
)

type cloneRequest struct {
	Name            *string                `json:"name"`
	DatasetID       *string                `json:"dataset_id"`
	TrainingConfig  json.RawMessage        `json:"training_config"`
	ResourceRequest *types.ResourceRequest `json:"resource_request"`
}

// The source snapshot stays on the server, including credentials. A frontend
// may submit changed fields after editing the reused configuration.
func clonedConfiguration(source *types.PipelineRun, request cloneRequest) (*types.PipelineRun, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var original types.PipelineRun
	if err := json.Unmarshal(encoded, &original); err != nil {
		return nil, err
	}
	out := &types.PipelineRun{UID: original.UID, ProjectID: original.ProjectID, DatasetID: original.DatasetID, TrainingConfig: original.TrainingConfig, ResourceRequest: original.ResourceRequest}
	if request.DatasetID != nil {
		out.DatasetID = *request.DatasetID
	}
	if request.ResourceRequest != nil {
		out.ResourceRequest = *request.ResourceRequest
	}
	if len(request.TrainingConfig) > 0 {
		var patch map[string]any
		if err := json.Unmarshal(request.TrainingConfig, &patch); err != nil || patch == nil {
			return nil, fmt.Errorf("training_config 必须是配置对象")
		}
		data, _ := json.Marshal(out.TrainingConfig)
		var merged map[string]any
		_ = json.Unmarshal(data, &merged)
		mergeCloneConfig(merged, patch)
		data, _ = json.Marshal(merged)
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&out.TrainingConfig); err != nil {
			return nil, fmt.Errorf("复用配置无效: %w", err)
		}
		if out.TrainingConfig.Distillation == nil {
			return nil, fmt.Errorf("distillation 配置不能为空")
		}
		if err := retainCloneCredentials(source.TrainingConfig.Distillation.TeacherModel, out.TrainingConfig.Distillation.TeacherModel, patch); err != nil {
			return nil, err
		}
	}
	if request.Name != nil {
		out.TrainingConfig.Distillation.Name = *request.Name
	} else {
		name := []rune(out.TrainingConfig.Distillation.Name)
		out.TrainingConfig.Distillation.Name = string(name[:min(len(name), 250)]) + "（副本）"
	}
	return out, nil
}

func mergeCloneConfig(destination, patch map[string]any) {
	for key, value := range patch {
		if nested, ok := value.(map[string]any); ok {
			base, _ := destination[key].(map[string]any)
			if base == nil {
				base = map[string]any{}
				destination[key] = base
			}
			mergeCloneConfig(base, nested)
		} else {
			destination[key] = value
		}
	}
}

func retainCloneCredentials(old, current *types.ModelConfig, patch map[string]any) error {
	if old == nil || current == nil {
		return nil
	}
	distillation, _ := patch["distillation"].(map[string]any)
	teacher, _ := distillation["teacher_model"].(map[string]any)
	// Moving from a deployed service to a manual endpoint must clear service_id.
	if _, endpointEdited := teacher["endpoint"]; endpointEdited && current.Endpoint != old.Endpoint {
		if _, explicitService := teacher["service_id"]; !explicitService {
			current.ServiceID = ""
		}
	}
	same := old.Endpoint == current.Endpoint && old.ModelName == current.ModelName && old.ServiceID == current.ServiceID
	_, supplied := teacher["api_secret_ref"]
	if current.APISecretRef == "********" {
		if !same {
			return fmt.Errorf("教师连接已变更，请重新输入密钥")
		}
		current.APISecretRef = old.APISecretRef
	} else if !same && !supplied {
		current.APISecretRef = ""
	}
	extra, _ := teacher["extra_params"].(map[string]any)
	for _, key := range []string{"api_key", "token", "password"} {
		value, exists := current.ExtraParams[key]
		if !exists {
			continue
		}
		_, supplied := extra[key]
		if value == "********" {
			if !same {
				return fmt.Errorf("教师连接已变更，请重新输入密钥")
			}
			current.ExtraParams[key] = old.ExtraParams[key]
		} else if !same && !supplied {
			delete(current.ExtraParams, key)
		}
	}
	return nil
}
