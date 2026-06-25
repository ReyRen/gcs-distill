package runtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultChatTemplate = `{{ message.content }}
{% if add_output and message.output is defined %}
{{ message.output }}
{% endif %}
`

type ManifestManager struct {
	storageRoot string
}

func NewManifestManager(storageRoot string) *ManifestManager {
	storageRoot = strings.TrimRight(strings.TrimSpace(storageRoot), "/")
	if storageRoot == "" {
		storageRoot = "/storage-root-jfs"
	}
	return &ManifestManager{storageRoot: storageRoot}
}

type Instruction struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output,omitempty"`
}

type LabeledData struct {
	Instruction string `json:"instruction"`
	Input       string `json:"input,omitempty"`
	Output      string `json:"output"`
	Teacher     string `json:"teacher,omitempty"`
}

type TrainingData struct {
	Instruction string  `json:"instruction"`
	Input       string  `json:"input,omitempty"`
	Output      string  `json:"output"`
	Quality     float64 `json:"quality,omitempty"`
}

func (m *ManifestManager) CreateSeedManifest(uid int, projectID, runID string, instructions []Instruction) error {
	seedPath := filepath.Join(m.runWorkspace(uid, projectID, runID), "data", "seed")
	if err := os.MkdirAll(seedPath, 0755); err != nil {
		return fmt.Errorf("create seed data directory: %w", err)
	}
	return writeJSONArray(filepath.Join(seedPath, "instructions.json"), instructions)
}

func (m *ManifestManager) CreateDefaultChatTemplate(uid int, projectID, runID string) (string, error) {
	templatePath := filepath.Join(m.runWorkspace(uid, projectID, runID), defaultTemplateRelPath)
	if err := os.MkdirAll(filepath.Dir(templatePath), 0755); err != nil {
		return "", fmt.Errorf("create chat template directory: %w", err)
	}
	if err := os.WriteFile(templatePath, []byte(defaultChatTemplate), 0644); err != nil {
		return "", fmt.Errorf("write chat template: %w", err)
	}
	return templatePath, nil
}

func (m *ManifestManager) LoadLabeledData(uid int, projectID, runID string) ([]LabeledData, error) {
	path := filepath.Join(m.runWorkspace(uid, projectID, runID), "data", "generated", "labeled.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read labeled data: %w", err)
	}

	var items []LabeledData
	if err := json.Unmarshal(data, &items); err == nil {
		return items, nil
	}
	if err := readJSONLines(data, &items); err != nil {
		return nil, fmt.Errorf("parse labeled data: %w", err)
	}
	return items, nil
}

func (m *ManifestManager) SaveFilteredData(uid int, projectID, runID string, trainData, testData []TrainingData) error {
	filteredPath := filepath.Join(m.runWorkspace(uid, projectID, runID), "data", "filtered")
	if err := os.MkdirAll(filteredPath, 0755); err != nil {
		return fmt.Errorf("create filtered data directory: %w", err)
	}
	if err := writeJSONArray(filepath.Join(filteredPath, "train.json"), trainData); err != nil {
		return fmt.Errorf("save train data: %w", err)
	}
	if err := writeJSONArray(filepath.Join(filteredPath, "test.json"), testData); err != nil {
		return fmt.Errorf("save test data: %w", err)
	}
	return nil
}

func (m *ManifestManager) GetManifestStats(uid int, projectID, runID string) (map[string]int, error) {
	base := m.runWorkspace(uid, projectID, runID)
	stats := map[string]int{}

	stats["seed"], _ = countJSONRecords(filepath.Join(base, "data", "seed", "instructions.json"))
	stats["labeled"], _ = countJSONRecords(filepath.Join(base, "data", "generated", "labeled.json"))
	stats["train"], _ = countJSONRecords(filepath.Join(base, "data", "filtered", "train.json"))
	stats["test"], _ = countJSONRecords(filepath.Join(base, "data", "filtered", "test.json"))

	return stats, nil
}

func (m *ManifestManager) runWorkspace(uid int, projectID, runID string) string {
	return filepath.Join(m.storageRoot, "user-"+strconv.Itoa(uid), "train-center", "model-distill", "projects", projectID, "runs", runID)
}

func writeJSONArray(path string, data interface{}) error {
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}

func readJSONLines[T any](data []byte, out *[]T) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var items []T
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	*out = items
	return nil
}

func countJSONRecords(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var records []json.RawMessage
	if err := json.Unmarshal(data, &records); err == nil {
		return len(records), nil
	}

	count := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) > 0 {
			count++
		}
	}
	return count, scanner.Err()
}
