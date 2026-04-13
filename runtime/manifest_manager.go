package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ManifestManager 清单管理器
type ManifestManager struct {
	workspaceRoot string
}

// NewManifestManager 创建清单管理器
func NewManifestManager(workspaceRoot string) *ManifestManager {
	return &ManifestManager{
		workspaceRoot: workspaceRoot,
	}
}

// Instruction 指令数据结构（种子数据）
type Instruction struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output,omitempty"`
}

// LabeledData 标注数据结构（教师推理输出）
type LabeledData struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output"`
	Teacher     string `json:"teacher,omitempty"`
}

// TrainingData 训练数据结构（治理后）
type TrainingData struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output"`
	Quality     float64 `json:"quality,omitempty"`
}

// CreateSeedManifest 创建种子数据清单
// 将用户上传的原始数据转换为 EasyDistill 期望的格式
func (m *ManifestManager) CreateSeedManifest(
	projectID, runID string,
	instructions []Instruction,
) error {
	seedPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "seed")

	// 确保目录存在
	if err := os.MkdirAll(seedPath, 0755); err != nil {
		return fmt.Errorf("创建种子数据目录失败: %w", err)
	}

	// 写入 instructions.json
	filePath := filepath.Join(seedPath, "instructions.json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建种子数据文件失败: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)

	for _, inst := range instructions {
		if err := encoder.Encode(inst); err != nil {
			return fmt.Errorf("写入指令数据失败: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("刷新数据文件失败: %w", err)
	}

	return nil
}

// LoadLabeledData 加载教师推理生成的标注数据
func (m *ManifestManager) LoadLabeledData(projectID, runID string) ([]LabeledData, error) {
	labeledPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "generated", "labeled.json")

	file, err := os.Open(labeledPath)
	if err != nil {
		return nil, fmt.Errorf("打开标注数据文件失败: %w", err)
	}
	defer file.Close()

	var data []LabeledData
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var item LabeledData
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, fmt.Errorf("解析标注数据失败: %w", err)
		}
		data = append(data, item)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取标注数据失败: %w", err)
	}

	return data, nil
}

// SaveFilteredData 保存治理后的训练数据
func (m *ManifestManager) SaveFilteredData(
	projectID, runID string,
	trainData, testData []TrainingData,
) error {
	filteredPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "filtered")

	// 确保目录存在
	if err := os.MkdirAll(filteredPath, 0755); err != nil {
		return fmt.Errorf("创建过滤数据目录失败: %w", err)
	}

	// 保存训练集
	if err := m.saveJSONL(filepath.Join(filteredPath, "train.json"), trainData); err != nil {
		return fmt.Errorf("保存训练数据失败: %w", err)
	}

	// 保存测试集
	if err := m.saveJSONL(filepath.Join(filteredPath, "test.json"), testData); err != nil {
		return fmt.Errorf("保存测试数据失败: %w", err)
	}

	return nil
}

// saveJSONL 保存 JSONL 格式文件
func (m *ManifestManager) saveJSONL(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)

	// 根据类型处理
	switch v := data.(type) {
	case []TrainingData:
		for _, item := range v {
			if err := encoder.Encode(item); err != nil {
				return err
			}
		}
	case []LabeledData:
		for _, item := range v {
			if err := encoder.Encode(item); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("不支持的数据类型")
	}

	return writer.Flush()
}

// GetManifestStats 获取清单统计信息
func (m *ManifestManager) GetManifestStats(projectID, runID string) (map[string]int, error) {
	stats := make(map[string]int)

	// 统计种子数据
	seedPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "seed", "instructions.json")
	seedCount, _ := m.countJSONLLines(seedPath)
	stats["seed"] = seedCount

	// 统计标注数据
	labeledPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "generated", "labeled.json")
	labeledCount, _ := m.countJSONLLines(labeledPath)
	stats["labeled"] = labeledCount

	// 统计训练数据
	trainPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "filtered", "train.json")
	trainCount, _ := m.countJSONLLines(trainPath)
	stats["train"] = trainCount

	// 统计测试数据
	testPath := filepath.Join(m.workspaceRoot, "projects", projectID, "runs", runID, "data", "filtered", "test.json")
	testCount, _ := m.countJSONLLines(testPath)
	stats["test"] = testCount

	return stats, nil
}

// countJSONLLines 统计 JSONL 文件行数
func (m *ManifestManager) countJSONLLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}

	return count, scanner.Err()
}
