package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func fillModelReadiness(m *LocalModel) {
	m.ModelType, m.SupportsVision, m.SuggestedTemplate = modelCapabilities(m.Path)
	err := validateOfflineModel(m.Path)
	m.Ready = err == nil
	if err != nil {
		m.ReadinessError = err.Error()
	}
}

func validateOfflineModel(path string) error {
	for _, name := range []string{"config.json", "tokenizer_config.json"} {
		if info, err := os.Stat(filepath.Join(path, name)); err != nil || info.IsDir() {
			return fmt.Errorf("离线学生模型缺少 %s", name)
		}
	}
	tokenizer := false
	for _, name := range []string{"tokenizer.json", "tokenizer.model", "vocab.json", "vocab.txt", "spiece.model"} {
		if info, err := os.Stat(filepath.Join(path, name)); err == nil && !info.IsDir() {
			tokenizer = true
		}
	}
	if !tokenizer {
		return fmt.Errorf("离线学生模型缺少 tokenizer 文件")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	weights := false
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".safetensors") || strings.HasPrefix(e.Name(), "pytorch_model") && strings.HasSuffix(e.Name(), ".bin")) {
			info, err := e.Info()
			if err == nil && info.Size() > 0 {
				weights = true
			}
		}
	}
	if !weights {
		return fmt.Errorf("离线学生模型缺少非空权重文件")
	}
	for _, name := range []string{"model.safetensors.index.json", "pytorch_model.bin.index.json"} {
		data, err := os.ReadFile(filepath.Join(path, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var index struct {
			WeightMap map[string]string `json:"weight_map"`
		}
		if json.Unmarshal(data, &index) != nil || len(index.WeightMap) == 0 {
			return fmt.Errorf("无效权重索引 %s", name)
		}
		for _, shard := range index.WeightMap {
			if filepath.Base(shard) != shard {
				return fmt.Errorf("无效权重分片路径")
			}
			if info, err := os.Stat(filepath.Join(path, shard)); err != nil || info.Size() == 0 {
				return fmt.Errorf("离线模型缺少权重分片 %s", shard)
			}
		}
	}
	return nil
}

func modelCapabilities(path string) (string, bool, string) {
	data, err := os.ReadFile(filepath.Join(path, "config.json"))
	if err != nil {
		return "", false, ""
	}
	var cfg struct {
		ModelType string          `json:"model_type"`
		Vision    json.RawMessage `json:"vision_config"`
	}
	if json.Unmarshal(data, &cfg) != nil {
		return "", false, ""
	}
	vision := len(cfg.Vision) > 0 && string(cfg.Vision) != "null"
	template := ""
	switch {
	case strings.HasPrefix(cfg.ModelType, "qwen2") && vision:
		template = "qwen2_vl"
	case strings.HasPrefix(cfg.ModelType, "qwen"):
		template = "qwen"
	case cfg.ModelType == "llama":
		template = "llama3"
	case cfg.ModelType == "mistral":
		template = "mistral"
	}
	return cfg.ModelType, vision, template
}
