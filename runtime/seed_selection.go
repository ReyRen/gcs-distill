package runtime

import (
	"context"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
)

type SeedSelection struct {
	SourceRecords   int    `json:"source_records"`
	SelectedRecords int    `json:"selected_records"`
	SeedLimit       int    `json:"seed_limit"`
	Method          string `json:"method"`
}

func InspectSeedSelection(ctx context.Context, path string, w *types.DistillationConfig) (*SeedSelection, error) {
	if w == nil || w.SeedLimit < 0 {
		return nil, fmt.Errorf("无效的种子选择配置")
	}
	result := &SeedSelection{SeedLimit: w.SeedLimit, Method: "all"}
	if w.SeedLimit > 0 {
		result.Method = "first_n"
	}
	total, err := WalkRecords(path, func(row map[string]interface{}) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if w.SeedLimit > 0 && result.SelectedRecords >= w.SeedLimit {
			return nil
		}
		if err := MapSeedRecord(row, w); err != nil {
			return err
		}
		result.SelectedRecords++
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.SourceRecords = total
	return result, nil
}

// Apply to each native generation section and pipeline operator, including
// operators whose embedded defaults do not explicitly declare retry_attempts.
func applyTeacherRetries(cfg map[string]interface{}, retries int) {
	for _, key := range []string{"generation", "cot"} {
		if section, ok := cfg[key].(map[string]interface{}); ok {
			section["retry_attempts"] = retries
		}
	}
	if stages, ok := cfg["pipeline"].([]interface{}); ok {
		for _, raw := range stages {
			stage, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			switch stage["stage"] {
			case "build_sft", "quality_filter", "difficulty_mix", "search_judge_filter", "instruct_eval", "cot_eval":
				continue
			}
			config, ok := stage["config"].(map[string]interface{})
			if !ok {
				config = map[string]interface{}{}
				stage["config"] = config
			}
			config["retry_attempts"] = retries
			for _, key := range []string{"generation", "solve", "judge", "rewrite"} {
				if child, ok := config[key].(map[string]interface{}); ok {
					child["retry_attempts"] = retries
				}
			}
		}
	}
}
