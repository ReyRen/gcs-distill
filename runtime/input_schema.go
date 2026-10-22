package runtime

import (
	"fmt"
	"strings"

	"github.com/ReyRen/gcs-distill/internal/types"
)

// MapSeedRecord adds canonical columns without dropping original messages,
// images, metadata, reference answers or tool traces.
func MapSeedRecord(row map[string]interface{}, w *types.DistillationConfig) error {
	for target, source := range w.FieldMapping {
		switch target {
		case "instruction", "problem", "response", "answer", "output", "images", "persona", "background", "messages":
		default:
			return fmt.Errorf("unsupported field_mapping target %q", target)
		}
		value, ok := row[source]
		if !ok {
			return fmt.Errorf("mapped input field %q is missing", source)
		}
		row[target] = value
	}
	text := func(keys ...string) bool {
		for _, key := range keys {
			if v, ok := row[key].(string); ok && strings.TrimSpace(v) != "" {
				return true
			}
		}
		return false
	}
	switch w.JobType {
	case "agent_distill":
		if !text("persona", "background") {
			return fmt.Errorf("Agent input requires persona or background")
		}
	case "search_agent_distill":
		if !text("instruction", "question", "query") || !text("answer", "output") {
			return fmt.Errorf("search Agent input requires a question and reference answer")
		}
	case "cot_distill", "advanced_cot_distill":
		if !text("problem", "instruction") {
			return fmt.Errorf("CoT input requires problem or instruction")
		}
	case "cot_long2short", "cot_short2long":
		if !text("problem", "instruction") || !text("response", "answer", "output") {
			return fmt.Errorf("CoT rewriting requires a problem and an existing response")
		}
	case "instruct_eval", "cot_eval":
		if row["messages"] != nil {
			return nil
		}
		if !text("instruction", "problem", "input") {
			return fmt.Errorf("quality evaluation requires an instruction or messages")
		}
		if !text("response", "answer", "output") && row["messages"] == nil {
			return fmt.Errorf("quality evaluation requires an existing response or messages")
		}
	case "mm_instruct_distill", "mm_cot_distill":
		if !text("instruction") {
			return fmt.Errorf("multimodal input requires instruction")
		}
		images, ok := row["images"].([]interface{})
		if !ok || len(images) == 0 {
			return fmt.Errorf("multimodal input requires a nonempty images array")
		}
		for _, raw := range images {
			reference, ok := raw.(string)
			if !ok || strings.TrimSpace(reference) == "" {
				return fmt.Errorf("图片引用必须是共享目录中的路径或内嵌图片")
			}
			if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
				return fmt.Errorf("离线图文蒸馏请先将图片上传到当前用户共享目录，不支持外链图片")
			}
		}
	default:
		if !text("instruction") {
			return fmt.Errorf("input requires instruction; configure field_mapping for another column")
		}
	}
	return nil
}
