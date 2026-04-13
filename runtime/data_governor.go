package runtime

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// DataGovernor 数据治理器
type DataGovernor struct {
	minLength     int     // 最小输出长度
	maxLength     int     // 最大输出长度
	qualityThresh float64 // 质量阈值（预留）
}

// NewDataGovernor 创建数据治理器
func NewDataGovernor() *DataGovernor {
	return &DataGovernor{
		minLength:     10,    // 最少10个字符
		maxLength:     4096,  // 最多4096个字符
		qualityThresh: 0.5,   // 质量分数阈值
	}
}

// SetLengthLimits 设置长度限制
func (g *DataGovernor) SetLengthLimits(min, max int) {
	g.minLength = min
	g.maxLength = max
}

// FilterData 数据治理主流程
// 对应阶段4: data_govern
func (g *DataGovernor) FilterData(labeled []LabeledData) (train []TrainingData, test []TrainingData, stats map[string]int) {
	stats = make(map[string]int)
	stats["total"] = len(labeled)
	stats["empty_removed"] = 0
	stats["length_invalid"] = 0
	stats["duplicates"] = 0

	// 去重map
	seen := make(map[string]bool)

	// 过滤后的数据
	var filtered []TrainingData

	for _, item := range labeled {
		// 1. 过滤空响应
		if g.isEmptyResponse(item.Output) {
			stats["empty_removed"]++
			continue
		}

		// 2. 长度校验
		if !g.isValidLength(item.Output) {
			stats["length_invalid"]++
			continue
		}

		// 3. 去重（基于 instruction + output 组合）
		key := g.makeKey(item.Instruction, item.Output)
		if seen[key] {
			stats["duplicates"]++
			continue
		}
		seen[key] = true

		// 4. 转换为训练数据格式
		trainData := TrainingData{
			Instruction: item.Instruction,
			Input:       item.Input,
			Output:      item.Output,
			Quality:     g.calculateQuality(item), // 质量评分（简化版本）
		}

		filtered = append(filtered, trainData)
	}

	stats["filtered"] = len(filtered)

	// 5. 划分训练集和测试集（90:10）
	splitIdx := int(float64(len(filtered)) * 0.9)
	train = filtered[:splitIdx]
	test = filtered[splitIdx:]

	stats["train"] = len(train)
	stats["test"] = len(test)

	return train, test, stats
}

// isEmptyResponse 检查是否为空响应
func (g *DataGovernor) isEmptyResponse(output string) bool {
	trimmed := strings.TrimSpace(output)

	// 完全为空
	if len(trimmed) == 0 {
		return true
	}

	// 常见的空响应模式
	emptyPatterns := []string{
		"无法回答",
		"抱歉",
		"我不知道",
		"I don't know",
		"I cannot",
		"N/A",
		"null",
		"None",
	}

	lowerOutput := strings.ToLower(trimmed)
	for _, pattern := range emptyPatterns {
		if strings.Contains(lowerOutput, strings.ToLower(pattern)) && len(trimmed) < 50 {
			return true
		}
	}

	return false
}

// isValidLength 检查长度是否有效
func (g *DataGovernor) isValidLength(output string) bool {
	length := utf8.RuneCountInString(output)
	return length >= g.minLength && length <= g.maxLength
}

// makeKey 生成去重键
func (g *DataGovernor) makeKey(instruction, output string) string {
	// 使用前100个字符生成键，避免键过长
	instKey := instruction
	if len(instKey) > 100 {
		instKey = instKey[:100]
	}

	outKey := output
	if len(outKey) > 100 {
		outKey = outKey[:100]
	}

	return fmt.Sprintf("%s|||%s", instKey, outKey)
}

// calculateQuality 计算质量分数（简化版本）
// 未来可以扩展为更复杂的质量评分模型
func (g *DataGovernor) calculateQuality(item LabeledData) float64 {
	score := 1.0

	// 长度惩罚（太短或太长都不好）
	length := utf8.RuneCountInString(item.Output)
	if length < 50 {
		score -= 0.2
	} else if length > 2000 {
		score -= 0.1
	}

	// 完整性奖励（有输入和输出）
	if item.Input != "" {
		score += 0.1
	}

	// 确保分数在 [0, 1] 范围内
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// GetFilterStats 格式化过滤统计信息
func (g *DataGovernor) GetFilterStats(stats map[string]int) string {
	return fmt.Sprintf(
		"数据治理完成 - 总数: %d, 空响应: %d, 长度不符: %d, 重复: %d, 保留: %d (训练: %d, 测试: %d)",
		stats["total"],
		stats["empty_removed"],
		stats["length_invalid"],
		stats["duplicates"],
		stats["filtered"],
		stats["train"],
		stats["test"],
	)
}
