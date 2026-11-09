package runtime

import (
	"math"
	"strconv"
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
	Path      string                   `json:"relative_path"`
	Records   int                      `json:"records"`
	Scored    int                      `json:"scored_records"`
	Unscored  int                      `json:"unscored_records"`
	Ambiguous int                      `json:"ambiguous_records"`
	Metrics   map[string]*ScoreSummary `json:"metrics"`
}

type QualityStatistics struct {
	ScoredArtifacts []ScoredArtifact `json:"scored_artifacts"`
	ExportedDataset *ScoredArtifact  `json:"exported_dataset"`
	Unavailable     []string         `json:"unavailable_artifacts"`
}

func (item *ScoredArtifact) add(key string, value interface{}) {
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
	m.Min = math.Min(m.Min, score)
	m.Max = math.Max(m.Max, score)
	m.Mean += (score - m.Mean) / float64(m.Count)
}

func summarizeQuality(root, relative string, index map[string]SampleQuality) (*ScoredArtifact, error) {
	path, err := SafeArtifactPath(root, relative)
	if err != nil {
		return nil, err
	}
	item := &ScoredArtifact{Path: relative, Metrics: map[string]*ScoreSummary{}}
	count, err := walkRecords(path, func(row map[string]interface{}) error {
		quality := sampleQuality(row, relative, index)
		candidates, _ := row["candidate_scores"].([]interface{})
		for _, score := range candidates {
			item.add("candidate_score", score)
		}
		for key, value := range quality.Metrics {
			item.add(key, value)
		}
		if chosen, a := quality.Metrics["chosen_score"].(float64); a {
			if rejected, b := quality.Metrics["rejected_score"].(float64); b {
				item.add("preference_margin", chosen-rejected)
			}
		}
		switch {
		case quality.Status == "scored" || len(candidates) > 0:
			item.Scored++
		case quality.Status == "ambiguous":
			item.Ambiguous++
		default:
			item.Unscored++
		}
		return nil
	}, true)
	if err != nil {
		return nil, err
	}
	item.Records = count
	return item, nil
}

func QualityReport(root string) (*QualityStatistics, error) {
	files, err := ListArtifacts(root)
	if err != nil {
		return nil, err
	}
	report := &QualityStatistics{ScoredArtifacts: []ScoredArtifact{}, Unavailable: []string{}}
	for _, file := range files {
		if file.RelativePath == "data/exported/dataset.jsonl" {
			path, err := SafeArtifactPath(root, file.RelativePath)
			if err != nil {
				return nil, err
			}
			index, err := exportSampleQuality(root, path)
			if err != nil {
				report.Unavailable = append(report.Unavailable, file.RelativePath)
				continue
			}
			report.ExportedDataset, err = summarizeQuality(root, file.RelativePath, index)
			if err != nil {
				report.Unavailable = append(report.Unavailable, file.RelativePath)
			}
		} else if scoreArtifact(file) {
			item, err := summarizeQuality(root, file.RelativePath, nil)
			if err != nil {
				report.Unavailable = append(report.Unavailable, file.RelativePath)
				continue
			}
			report.ScoredArtifacts = append(report.ScoredArtifacts, *item)
		}
	}
	return report, nil
}
