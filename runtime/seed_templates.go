package runtime

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// Templates are shipped with the binary so downloads work in offline deployments.
// They are format examples, not registered datasets or business training records.
//
//go:embed seed_templates
var seedTemplateFiles embed.FS

type SeedTemplate struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	JobTypes       []string                 `json:"job_types"`
	RequiredFields []string                 `json:"required_fields"`
	Filename       string                   `json:"filename"`
	RecordCount    int                      `json:"record_count"`
	ExampleRecords []map[string]interface{} `json:"example_records"`
	DownloadPath   string                   `json:"download_path"`
	ExamplesOnly   bool                     `json:"examples_only"`
}

func SeedTemplates() ([]SeedTemplate, error) {
	items := []SeedTemplate{
		{ID: "instruction", Name: "指令与偏好数据种子", Description: "提供问题或指令，由教师生成回答；DPO 的候选回答与偏好对由任务生成。", RequiredFields: []string{"instruction"}, JobTypes: []string{"instruct_distill", "advanced_instruct_distill", "augmented_instruct_distill", "balanced_instruct_distill", "dpo_data_build"}},
		{ID: "reasoning", Name: "思维链生成种子", Description: "提供待解题目，由教师生成推理过程与答案。", RequiredFields: []string{"problem"}, JobTypes: []string{"cot_distill", "advanced_cot_distill"}},
		{ID: "reasoning-rewrite", Name: "思维链压缩与扩展种子", Description: "同时提供题目和已有推理回答，用于压缩或扩展。", RequiredFields: []string{"problem", "answer"}, JobTypes: []string{"cot_long2short", "cot_short2long"}},
		{ID: "evaluation", Name: "已有回答质量评估样例", Description: "同时提供问题和已有回答；只评估已有数据，不生成学生模型。", RequiredFields: []string{"instruction", "output"}, JobTypes: []string{"instruct_eval", "cot_eval"}},
		{ID: "agent", Name: "智能体角色与背景种子", Description: "提供角色、职责和场景背景；原生配方中的模拟工具不代表已连接真实业务系统。", RequiredFields: []string{"persona"}, JobTypes: []string{"agent_distill"}},
		{ID: "search-agent", Name: "搜索智能体问答种子", Description: "同时提供问题和可核对的参考答案；离线使用配方的模拟搜索模式。", RequiredFields: []string{"question", "answer"}, JobTypes: []string{"search_agent_distill"}},
		{ID: "multimodal", Name: "多模态图文蒸馏种子", Description: "包含问题和内嵌 PNG 示意图，下载后可直接上传；需要支持视觉输入的教师。", RequiredFields: []string{"instruction", "images"}, JobTypes: []string{"mm_instruct_distill", "mm_cot_distill"}},
	}
	for i := range items {
		item := &items[i]
		item.Filename = fmt.Sprintf("%02d_%s.jsonl", i+1, item.Name)
		item.DownloadPath = "dataset-templates/" + item.ID + "/download"
		item.ExamplesOnly = true
		data, err := seedTemplateFiles.ReadFile("seed_templates/" + item.Filename)
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
			var row map[string]interface{}
			if err := json.Unmarshal(line, &row); err != nil || row == nil {
				return nil, fmt.Errorf("invalid seed template %s", item.ID)
			}
			item.RecordCount++
			if len(item.ExampleRecords) == 0 {
				item.ExampleRecords = append(item.ExampleRecords, row)
			}
		}
	}
	return items, nil
}

func ReadSeedTemplate(id string) (SeedTemplate, []byte, error) {
	items, err := SeedTemplates()
	if err != nil {
		return SeedTemplate{}, nil, err
	}
	for _, item := range items {
		if item.ID == id {
			data, err := seedTemplateFiles.ReadFile("seed_templates/" + item.Filename)
			return item, data, err
		}
	}
	return SeedTemplate{}, nil, fs.ErrNotExist
}

func SeedTemplateBundle() ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	err := fs.WalkDir(seedTemplateFiles, "seed_templates", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := seedTemplateFiles.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := w.Create(strings.TrimPrefix(path, "seed_templates/"))
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		return err
	})
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
