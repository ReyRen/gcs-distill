package mysql

import (
	"github.com/ReyRen/gcs-distill/internal/types"
	"strings"
)

func listConditions(uid int, projectID, kind string, filters []types.ListFilter) (string, []interface{}) {
	clauses := []string{"uid = ?"}
	args := []interface{}{uid}
	if projectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if len(filters) > 0 {
		f := filters[0]
		if f.Name != "" {
			column := "name"
			if kind == "pipeline" {
				column = "COALESCE(JSON_UNQUOTE(JSON_EXTRACT(training_config,'$.distillation.name')), '')"
			}
			clauses = append(clauses, column+" LIKE ? ESCAPE '!'")
			args = append(args, "%"+strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(f.Name)+"%")
		}
		if f.Status != "" && kind == "pipeline" {
			clauses = append(clauses, "status = ?")
			args = append(args, f.Status)
		}
		if f.SourceType != "" && kind == "dataset" {
			clauses = append(clauses, "source_type = ?")
			args = append(args, f.SourceType)
		}
		if f.JobType != "" && kind == "pipeline" {
			clauses = append(clauses, "JSON_UNQUOTE(JSON_EXTRACT(training_config,'$.distillation.job_type')) = ?")
			args = append(args, f.JobType)
		}
	}
	return strings.Join(clauses, " AND "), args
}
