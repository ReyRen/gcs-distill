package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// ReadTrainerMetrics exposes results actually written by LLaMA-Factory. A run
// that requested validation must produce a finite validation loss.
func ReadTrainerMetrics(dir string, validation bool) (map[string]interface{}, error) {
	metrics := map[string]interface{}{}
	for _, name := range []string{"train_results.json", "eval_results.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		required := validation && name == "eval_results.json"
		if err != nil {
			if required {
				return nil, fmt.Errorf("学生验证未产出 eval_results.json: %w", err)
			}
			continue
		}
		var values map[string]interface{}
		if err := json.Unmarshal(data, &values); err != nil {
			if required {
				return nil, fmt.Errorf("学生验证结果不是有效 JSON: %w", err)
			}
			continue
		}
		for _, key := range []string{"train_loss", "eval_loss", "train_runtime", "eval_runtime"} {
			if value, ok := values[key].(float64); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
				metrics[key] = value
			}
		}
	}
	if validation && metrics["eval_loss"] == nil {
		return nil, fmt.Errorf("已要求学生验证，但没有有效的 eval_loss；请检查学生训练日志")
	}
	return metrics, nil
}
