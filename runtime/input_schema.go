package runtime

import (
	"fmt"
	"sort"
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
	if SupportsDataExpansion(w.JobType) && !text("instruction") && w.FieldMapping["instruction"] == "" && row["messages"] != nil {
		if images, exists := row["images"]; exists && images != nil {
			values, ok := images.([]interface{})
			if !ok || len(values) > 0 {
				return fmt.Errorf("含图片的 SFT 数据请使用多模态配方并提供 instruction、images，不能自动转换为纯文本种子")
			}
		}
		instruction, err := singleTurnSeedInstruction(row["messages"])
		if err != nil {
			return err
		}
		row["instruction"] = instruction
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
			fields := make([]string, 0, len(row))
			for key := range row {
				fields = append(fields, key)
			}
			sort.Strings(fields)
			return fmt.Errorf("需要非空的 instruction 文本，当前字段为 [%s]；其他问题列请配置 field_mapping，例如 {\"instruction\":\"question\"}", strings.Join(fields, ", "))
		}
	}
	return nil
}

// Reusing an exported single-turn SFT dataset as instruction seeds is a data
// conversion, not a replay of its old answers. Keep the source messages intact.
func singleTurnSeedInstruction(raw interface{}) (string, error) {
	messages, ok := raw.([]interface{})
	if !ok || len(messages) == 0 {
		return "", fmt.Errorf("messages 必须是非空的单轮对话数组")
	}
	var instruction string
	systemSeen, userSeen, assistantSeen := false, false, false
	for _, raw := range messages {
		message, ok := raw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("messages 中每条消息必须是对象")
		}
		content, text := message["content"].(string)
		if !text {
			return "", fmt.Errorf("指令种子只支持文本消息，图文内容请使用多模态配方并提供 instruction、images")
		}
		if message["tool_calls"] != nil {
			return "", fmt.Errorf("工具对话不能直接转换为指令种子，请先整理为 instruction 文本或选择 Agent 配方")
		}
		switch message["role"] {
		case "system":
			if systemSeen || userSeen || strings.TrimSpace(content) == "" {
				return "", fmt.Errorf("messages 应为可选 system、一个 user、可选 assistant 的单轮对话")
			}
			systemSeen = true
		case "user":
			if userSeen || assistantSeen {
				return "", fmt.Errorf("多轮 messages 不能自动转换为单条指令，请先整理为 instruction 文本")
			}
			instruction = strings.TrimSpace(content)
			if instruction == "" {
				return "", fmt.Errorf("messages 的 user 消息不能为空")
			}
			userSeen = true
		case "assistant":
			if !userSeen || assistantSeen {
				return "", fmt.Errorf("messages 应为可选 system、一个 user、可选 assistant 的单轮对话")
			}
			assistantSeen = true
		default:
			return "", fmt.Errorf("messages 含不支持的角色，仅支持单轮 system/user/assistant 文本对话")
		}
	}
	if !userSeen {
		return "", fmt.Errorf("messages 缺少 user 问题，无法生成 instruction")
	}
	return instruction, nil
}
