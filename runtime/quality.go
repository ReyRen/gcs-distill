package runtime

import (
	"math"
	"strconv"
	"strings"
)

type ScoreSummary struct {
	Histogram         map[string]int `json:"histogram"`
	HistogramBinWidth float64        `json:"histogram_bin_width"`
	Count             int            `json:"count"`
	Min               float64        `json:"min"`
	Max               float64        `json:"max"`
	Mean              float64        `json:"mean"`
}
type ScoredArtifact struct {
	Path    string                   `json:"relative_path"`
	Records int                      `json:"records"`
	Metrics map[string]*ScoreSummary `json:"metrics"`
}

func QualityReport(root string) ([]ScoredArtifact, error) {
	files, err := ListArtifacts(root)
	if err != nil {
		return nil, err
	}
	report := []ScoredArtifact{}
	for _, file := range files {
		if file.Kind != "dataset" || !strings.HasPrefix(file.RelativePath, "data/intermediate/") {
			continue
		}
		if !strings.Contains(file.Name, "score") && !strings.Contains(file.Name, "eval") && !strings.Contains(file.Name, "pairs") {
			continue
		}
		path, err := SafeArtifactPath(root, file.RelativePath)
		if err != nil {
			return nil, err
		}
		item := ScoredArtifact{Path: file.RelativePath, Metrics: map[string]*ScoreSummary{}}
		add := func(key string, value any) {
			var score float64
			switch v := value.(type) {
			case float64:
				score = v
			case bool:
				if v {
					score = 1
				}
			default:
				return
			}
			m := item.Metrics[key]
			if m == nil {
				m = &ScoreSummary{Min: score, Max: score, Histogram: map[string]int{}, HistogramBinWidth: 0.1}
				item.Metrics[key] = m
			}
			m.Count++
			bin := strconv.FormatFloat(math.Floor(score*10)/10, 'f', 1, 64)
			if len(m.Histogram) >= 1000 && m.Histogram[bin] == 0 {
				bin = "other"
			}
			m.Histogram[bin]++
			if score < m.Min {
				m.Min = score
			}
			if score > m.Max {
				m.Max = score
			}
			m.Mean += (score - m.Mean) / float64(m.Count)
		}
		count, err := WalkRecords(path, func(row map[string]interface{}) error {
			if scores, ok := row["candidate_scores"].([]interface{}); ok {
				for _, score := range scores {
					add("candidate_score", score)
				}
			}
			for _, key := range []string{"helpfulness", "informativeness", "correctness", "generalization", "reasoning_verbosity", "cognitive_difficulty", "logical_correctness", "chosen_score", "rejected_score"} {
				add(key, row[key])
			}
			for _, group := range []string{"scores", "metrics", "eval_results", "evaluation"} {
				if values, ok := row[group].(map[string]interface{}); ok {
					for key, value := range values {
						if len(item.Metrics) >= 100 {
							break
						}
						if obj, ok := value.(map[string]interface{}); ok {
							value = obj["score"]
						}
						add(key, value)
					}
				}
			}
			chosen, a := row["chosen_score"].(float64)
			rejected, b := row["rejected_score"].(float64)
			if a && b {
				add("preference_margin", chosen-rejected)
			}
			return nil
		})
		if err != nil {
			continue
		}
		item.Records = count
		report = append(report, item)
	}
	return report, nil
}
