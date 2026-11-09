package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
)

// SampleQuality contains recorded teacher scores, never an inferred overall
// grade. Boolean metrics remain boolean, including a real correctness=false.
type SampleQuality struct {
	Status  string                 `json:"status"`
	Metrics map[string]interface{} `json:"metrics,omitempty"`
	Sources []string               `json:"sources,omitempty"`
}

type PreviewQuality struct {
	Scope     string          `json:"scope"`
	Scored    int             `json:"scored_records"`
	Unscored  int             `json:"unscored_records"`
	Ambiguous int             `json:"ambiguous_records"`
	Items     []SampleQuality `json:"items"`
}

func recordScores(row map[string]interface{}) map[string]interface{} {
	scores := map[string]interface{}{}
	add := func(key string, value interface{}) {
		if obj, ok := value.(map[string]interface{}); ok {
			value = obj["score"]
		}
		switch value.(type) {
		case float64, bool:
			scores[key] = value
		}
	}
	for _, key := range []string{"helpfulness", "informativeness", "correctness", "generalization", "reasoning_verbosity", "cognitive_difficulty", "logical_correctness", "chosen_score", "rejected_score"} {
		add(key, row[key])
	}
	for _, group := range []string{"scores", "metrics", "eval_results", "evaluation"} {
		if values, ok := row[group].(map[string]interface{}); ok {
			for key, value := range values {
				if len(scores) < 100 {
					add(key, value)
				}
			}
		}
	}
	if quality, ok := fieldValue(row, "metadata.quality").(map[string]interface{}); ok && quality["status"] == "scored" {
		if values, ok := quality["metrics"].(map[string]interface{}); ok {
			for key, value := range values {
				add(key, value)
			}
		}
	}
	return scores
}

// Only an exact single-turn question AND answer can restore stripped SFT
// scores. IDs repeat between expansion rounds; prompts alone are not unique.
// Multimodal/tool/multi-turn records require their own stored score metadata.
func scoreIdentity(row map[string]interface{}) string {
	if images, ok := row["images"]; ok && images != nil {
		if list, ok := images.([]interface{}); !ok || len(list) > 0 {
			return ""
		}
	}
	var question, answer string
	if messages, ok := row["messages"].([]interface{}); ok {
		for i, raw := range messages {
			m, ok := raw.(map[string]interface{})
			if !ok || m["tool_calls"] != nil {
				return ""
			}
			text, ok := m["content"].(string)
			if !ok {
				return ""
			}
			switch m["role"] {
			case "system":
				if i != 0 {
					return ""
				}
			case "user":
				if question != "" || answer != "" {
					return ""
				}
				question = text
			case "assistant":
				if question == "" || answer != "" {
					return ""
				}
				answer = text
			default:
				return ""
			}
		}
	} else {
		if input, _ := row["input"].(string); strings.TrimSpace(input) != "" {
			return ""
		}
		question = firstString(stringField(row, "instruction"), stringField(row, "problem"))
		answer = firstString(stringField(row, "response"), stringField(row, "output"))
	}
	if strings.TrimSpace(question) == "" || strings.TrimSpace(answer) == "" {
		return ""
	}
	b, _ := json.Marshal([]string{strings.TrimSpace(question), strings.TrimSpace(answer)})
	return digest(b)
}

func stringField(row map[string]interface{}, key string) string {
	v, _ := row[key].(string)
	return v
}

func scoreArtifact(file Artifact) bool {
	if file.Kind != "dataset" || !strings.HasPrefix(file.RelativePath, "data/intermediate/") {
		return false
	}
	for _, name := range []string{"score", "eval", "pairs", "quality_filter", "rv_cd", "rubrics", "judge_filter"} {
		if strings.Contains(file.Name, name) {
			return true
		}
	}
	return false
}

// Read each candidate file atomically into a temporary index; malformed files
// contribute no partial scores. The index is limited to the requested samples.
func loadSampleQuality(root string, keys map[string]bool) (map[string]SampleQuality, error) {
	result := map[string]SampleQuality{}
	if len(keys) == 0 {
		return result, nil
	}
	files, err := ListArtifacts(root)
	if err != nil {
		return nil, err
	}
	merge := func(dst map[string]SampleQuality, key string, quality SampleQuality) {
		old, exists := dst[key]
		if !exists {
			dst[key] = quality
			return
		}
		for _, source := range quality.Sources {
			found := false
			for _, previous := range old.Sources {
				found = found || previous == source
			}
			if !found {
				old.Sources = append(old.Sources, source)
			}
		}
		if old.Status == "ambiguous" || quality.Status == "ambiguous" || !reflect.DeepEqual(old.Metrics, quality.Metrics) {
			old.Status, old.Metrics = "ambiguous", nil
		}
		dst[key] = old
	}
	for _, file := range files {
		if !scoreArtifact(file) {
			continue
		}
		path, err := SafeArtifactPath(root, file.RelativePath)
		if err != nil {
			return nil, err
		}
		candidate := map[string]SampleQuality{}
		_, err = walkRecords(path, func(row map[string]interface{}) error {
			key := scoreIdentity(row)
			if keys[key] {
				if metrics := recordScores(row); len(metrics) > 0 {
					merge(candidate, key, SampleQuality{Status: "scored", Metrics: metrics, Sources: []string{file.RelativePath}})
				}
			}
			return nil
		}, true)
		if err != nil {
			continue
		}
		for key, quality := range candidate {
			merge(result, key, quality)
		}
	}
	return result, nil
}

func sampleQuality(row map[string]interface{}, path string, index map[string]SampleQuality) SampleQuality {
	if metrics := recordScores(row); len(metrics) > 0 {
		quality := SampleQuality{Status: "scored", Metrics: metrics, Sources: []string{path}}
		if sources, ok := fieldValue(row, "metadata.quality.sources").([]interface{}); ok && len(sources) > 0 {
			quality.Sources = nil
			for _, raw := range sources {
				if source, ok := raw.(string); ok {
					quality.Sources = append(quality.Sources, source)
				}
			}
		}
		return quality
	}
	if quality, ok := index[scoreIdentity(row)]; ok {
		return quality
	}
	return SampleQuality{Status: "unavailable"}
}

func PreviewArtifact(root, relative string, limit int) ([]map[string]interface{}, int, []string, *PreviewQuality, error) {
	path, err := SafeArtifactPath(root, relative)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	rows, total, fields, err := previewRecords(path, limit, true)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	keys := map[string]bool{}
	for _, row := range rows {
		if len(recordScores(row)) == 0 {
			if key := scoreIdentity(row); key != "" {
				keys[key] = true
			}
		}
	}
	index, err := loadSampleQuality(root, keys)
	if err != nil {
		return nil, 0, nil, nil, err
	}
	quality := &PreviewQuality{Scope: "preview_records", Items: []SampleQuality{}}
	for _, row := range rows {
		q := sampleQuality(row, relative, index)
		quality.Items = append(quality.Items, q)
		switch q.Status {
		case "scored":
			quality.Scored++
		case "ambiguous":
			quality.Ambiguous++
		default:
			quality.Unscored++
		}
	}
	return rows, total, fields, quality, nil
}

// New exports persist restored scores before the immutable split snapshots and
// reusable dataset asset are published. Existing files are never rewritten by
// preview or quality endpoints.
func exportSampleQuality(root, path string) (map[string]SampleQuality, error) {
	keys := map[string]bool{}
	_, err := WalkRecords(path, func(row map[string]interface{}) error {
		if len(recordScores(row)) == 0 {
			if key := scoreIdentity(row); key != "" {
				keys[key] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loadSampleQuality(root, keys)
}
