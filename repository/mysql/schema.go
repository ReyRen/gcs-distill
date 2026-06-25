package mysql

import (
	"context"
	"strings"
)

const distillSchema = `
CREATE TABLE IF NOT EXISTS distill_projects (
  id VARCHAR(64) PRIMARY KEY,
  uid BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  business_scenario VARCHAR(255),
  teacher_model_config JSON NOT NULL,
  student_model_config JSON NOT NULL,
  evaluation_config JSON,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  INDEX idx_distill_projects_uid_created_at (uid, created_at),
  INDEX idx_distill_projects_name (name),
  INDEX idx_distill_projects_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS distill_datasets (
  id VARCHAR(64) PRIMARY KEY,
  uid BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  source_type VARCHAR(50) NOT NULL,
  file_path TEXT NOT NULL,
  record_count INT DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_distill_datasets_uid_created_at (uid, created_at),
  INDEX idx_distill_datasets_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS distill_pipeline_runs (
  id VARCHAR(64) PRIMARY KEY,
  uid BIGINT NOT NULL,
  project_id VARCHAR(64) NOT NULL,
  dataset_id VARCHAR(64) NOT NULL,
  status VARCHAR(50) NOT NULL DEFAULT 'pending',
  current_stage INT DEFAULT 0,
  trigger_mode VARCHAR(50) DEFAULT 'manual',
  training_config JSON NOT NULL,
  resource_request JSON,
  error_message TEXT,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  INDEX idx_distill_pipeline_runs_uid_project (uid, project_id),
  INDEX idx_distill_pipeline_runs_project_id (project_id),
  INDEX idx_distill_pipeline_runs_status (status),
  INDEX idx_distill_pipeline_runs_created_at (created_at),
  CONSTRAINT fk_distill_pipeline_project FOREIGN KEY (project_id) REFERENCES distill_projects(id) ON DELETE CASCADE,
  CONSTRAINT fk_distill_pipeline_dataset FOREIGN KEY (dataset_id) REFERENCES distill_datasets(id)
);

CREATE TABLE IF NOT EXISTS distill_stage_runs (
  id VARCHAR(64) PRIMARY KEY,
  pipeline_run_id VARCHAR(64) NOT NULL,
  stage_type VARCHAR(50) NOT NULL,
  stage_order INT NOT NULL,
  status VARCHAR(50) NOT NULL DEFAULT 'pending',
  container_id VARCHAR(255),
  node_name VARCHAR(255),
  config_path TEXT,
  input_manifest JSON,
  output_manifest JSON,
  metrics JSON,
  log_path TEXT,
  retry_count INT DEFAULT 0,
  error_message TEXT,
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  INDEX idx_distill_stage_runs_pipeline_run_id (pipeline_run_id),
  INDEX idx_distill_stage_runs_stage_type (stage_type),
  INDEX idx_distill_stage_runs_status (status),
  INDEX idx_distill_stage_runs_stage_order (pipeline_run_id, stage_order),
  CONSTRAINT fk_distill_stage_pipeline FOREIGN KEY (pipeline_run_id) REFERENCES distill_pipeline_runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS distill_container_runs (
  id VARCHAR(64) PRIMARY KEY,
  stage_run_id VARCHAR(64) NOT NULL,
  container_name VARCHAR(255) NOT NULL,
  image VARCHAR(255) NOT NULL,
  node_name VARCHAR(255) NOT NULL,
  node_addr VARCHAR(255) NOT NULL,
  command TEXT,
  args JSON,
  envs JSON,
  mounts JSON,
  xpu_allocation VARCHAR(255),
  exit_code INT,
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_distill_container_runs_stage_run_id (stage_run_id),
  INDEX idx_distill_container_runs_node_name (node_name),
  INDEX idx_distill_container_runs_created_at (created_at),
  CONSTRAINT fk_distill_container_stage FOREIGN KEY (stage_run_id) REFERENCES distill_stage_runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS distill_evaluation_reports (
  id VARCHAR(64) PRIMARY KEY,
  pipeline_run_id VARCHAR(64) NOT NULL,
  stage_run_id VARCHAR(64) NOT NULL,
  metrics JSON NOT NULL,
  details JSON,
  summary TEXT,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_distill_evaluation_reports_pipeline_run_id (pipeline_run_id),
  INDEX idx_distill_evaluation_reports_stage_run_id (stage_run_id),
  INDEX idx_distill_evaluation_reports_created_at (created_at),
  CONSTRAINT fk_distill_eval_pipeline FOREIGN KEY (pipeline_run_id) REFERENCES distill_pipeline_runs(id) ON DELETE CASCADE,
  CONSTRAINT fk_distill_eval_stage FOREIGN KEY (stage_run_id) REFERENCES distill_stage_runs(id)
);

CREATE TABLE IF NOT EXISTS distill_artifacts (
  id VARCHAR(64) PRIMARY KEY,
  pipeline_run_id VARCHAR(64) NOT NULL,
  stage_run_id VARCHAR(64),
  artifact_type VARCHAR(50) NOT NULL,
  name VARCHAR(255) NOT NULL,
  file_path TEXT NOT NULL,
  file_size BIGINT,
  metadata JSON,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  INDEX idx_distill_artifacts_pipeline_run_id (pipeline_run_id),
  INDEX idx_distill_artifacts_stage_run_id (stage_run_id),
  INDEX idx_distill_artifacts_artifact_type (artifact_type),
  INDEX idx_distill_artifacts_created_at (created_at),
  CONSTRAINT fk_distill_artifact_pipeline FOREIGN KEY (pipeline_run_id) REFERENCES distill_pipeline_runs(id) ON DELETE CASCADE,
  CONSTRAINT fk_distill_artifact_stage FOREIGN KEY (stage_run_id) REFERENCES distill_stage_runs(id)
);
`

func (db *DB) EnsureSchema(ctx context.Context) error {
	for _, stmt := range splitSQLStatements(distillSchema) {
		if _, err := db.sql.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSQLStatements(schema string) []string {
	parts := strings.Split(schema, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}
