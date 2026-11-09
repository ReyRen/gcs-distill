package runtime

import (
	"context"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
)

type WorkflowCompatibility struct {
	JobType       string   `json:"job_type"`
	Name          string   `json:"name"`
	Compatible    bool     `json:"compatible"`
	Reason        string   `json:"reason,omitempty"`
	InvalidRecord int      `json:"invalid_record,omitempty"`
	InputFields   []string `json:"input_fields"`
	OutputFormat  string   `json:"output_format"`
}

type DatasetCompatibility struct {
	SeedSelection SeedSelection           `json:"seed_selection"`
	Items         []WorkflowCompatibility `json:"items"`
}

// Read the file once, checking every selected record against all recipes using
// the same normalizer as task creation. Per-recipe copies prevent cross-talk.
func InspectWorkflowCompatibility(ctx context.Context, path string, limit int) (*DatasetCompatibility, error) {
	if limit < 0 {
		return nil, fmt.Errorf("seed_limit 不能为负数")
	}
	result := &DatasetCompatibility{SeedSelection: SeedSelection{SeedLimit: limit, Method: "all"}, Items: []WorkflowCompatibility{}}
	if limit > 0 {
		result.SeedSelection.Method = "first_n"
	}
	for _, recipe := range WorkflowRecipes() {
		result.Items = append(result.Items, WorkflowCompatibility{JobType: recipe.JobType, Name: recipe.Name, Compatible: true, InputFields: recipe.InputFields, OutputFormat: recipe.OutputFormat})
	}
	total, err := WalkRecords(path, func(row map[string]interface{}) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if limit > 0 && result.SeedSelection.SelectedRecords >= limit {
			return nil
		}
		result.SeedSelection.SelectedRecords++
		for i := range result.Items {
			item := &result.Items[i]
			if !item.Compatible {
				continue
			}
			copy := map[string]interface{}{}
			for key, value := range row {
				copy[key] = value
			}
			if err := MapSeedRecord(copy, &types.DistillationConfig{JobType: item.JobType}); err != nil {
				item.Compatible = false
				item.Reason = err.Error()
				item.InvalidRecord = result.SeedSelection.SelectedRecords
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.SeedSelection.SourceRecords = total
	return result, nil
}
