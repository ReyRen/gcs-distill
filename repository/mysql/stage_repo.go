package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/google/uuid"
)

type StageRepository interface {
	Create(ctx context.Context, stage *types.StageRun) error
	GetByID(ctx context.Context, id string) (*types.StageRun, error)
	ListByPipeline(ctx context.Context, pipelineID string) ([]*types.StageRun, error)
	Update(ctx context.Context, stage *types.StageRun) error
	UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	Delete(ctx context.Context, id string) error
}

type stageRepo struct {
	db *DB
}

func NewStageRepository(db *DB) StageRepository {
	return &stageRepo{db: db}
}

func (r *stageRepo) Create(ctx context.Context, stage *types.StageRun) error {
	if stage.ID == "" {
		stage.ID = uuid.NewString()
	}
	now := time.Now()
	stage.CreatedAt = now
	stage.UpdatedAt = now

	inputManifest, outputManifest, metrics, err := stageJSON(stage)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO distill_stage_runs (
			id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := r.db.sql.ExecContext(ctx, query,
		stage.ID,
		stage.PipelineRunID,
		stage.StageType,
		stage.StageOrder,
		stage.Status,
		stage.ContainerID,
		stage.NodeName,
		stage.ConfigPath,
		inputManifest,
		outputManifest,
		metrics,
		stage.LogPath,
		stage.RetryCount,
		stage.CreatedAt,
		stage.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create stage run failed: %w", err)
	}
	return nil
}

func (r *stageRepo) GetByID(ctx context.Context, id string) (*types.StageRun, error) {
	query := `
		SELECT id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count, error_message,
			started_at, finished_at, created_at, updated_at
		FROM distill_stage_runs
		WHERE id = ?
	`

	var stage types.StageRun
	var inputManifest, outputManifest, metrics []byte
	var containerID, nodeName, configPath, logPath, errorMessage sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := r.db.sql.QueryRowContext(ctx, query, id).Scan(
		&stage.ID,
		&stage.PipelineRunID,
		&stage.StageType,
		&stage.StageOrder,
		&stage.Status,
		&containerID,
		&nodeName,
		&configPath,
		&inputManifest,
		&outputManifest,
		&metrics,
		&logPath,
		&stage.RetryCount,
		&errorMessage,
		&startedAt,
		&finishedAt,
		&stage.CreatedAt,
		&stage.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("stage run not found: %s", id)
		}
		return nil, fmt.Errorf("query stage run failed: %w", err)
	}
	if err := scanStageJSON(&stage, inputManifest, outputManifest, metrics); err != nil {
		return nil, err
	}
	stage.ContainerID = nullStringValue(containerID)
	stage.NodeName = nullStringValue(nodeName)
	stage.ConfigPath = nullStringValue(configPath)
	stage.LogPath = nullStringValue(logPath)
	stage.ErrorMessage = nullStringValue(errorMessage)
	stage.StartedAt = nullTimePtr(startedAt)
	stage.FinishedAt = nullTimePtr(finishedAt)
	return &stage, nil
}

func (r *stageRepo) ListByPipeline(ctx context.Context, pipelineID string) ([]*types.StageRun, error) {
	query := `
		SELECT id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count, error_message,
			started_at, finished_at, created_at, updated_at
		FROM distill_stage_runs
		WHERE pipeline_run_id = ?
		ORDER BY stage_order ASC
	`

	rows, err := r.db.sql.QueryContext(ctx, query, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("query stage run list failed: %w", err)
	}
	defer rows.Close()

	var stages []*types.StageRun
	for rows.Next() {
		var stage types.StageRun
		var inputManifest, outputManifest, metrics []byte
		var containerID, nodeName, configPath, logPath, errorMessage sql.NullString
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&stage.ID,
			&stage.PipelineRunID,
			&stage.StageType,
			&stage.StageOrder,
			&stage.Status,
			&containerID,
			&nodeName,
			&configPath,
			&inputManifest,
			&outputManifest,
			&metrics,
			&logPath,
			&stage.RetryCount,
			&errorMessage,
			&startedAt,
			&finishedAt,
			&stage.CreatedAt,
			&stage.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan stage row failed: %w", err)
		}
		if err := scanStageJSON(&stage, inputManifest, outputManifest, metrics); err != nil {
			return nil, err
		}
		stage.ContainerID = nullStringValue(containerID)
		stage.NodeName = nullStringValue(nodeName)
		stage.ConfigPath = nullStringValue(configPath)
		stage.LogPath = nullStringValue(logPath)
		stage.ErrorMessage = nullStringValue(errorMessage)
		stage.StartedAt = nullTimePtr(startedAt)
		stage.FinishedAt = nullTimePtr(finishedAt)
		stages = append(stages, &stage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stage rows failed: %w", err)
	}
	return stages, nil
}

func (r *stageRepo) Update(ctx context.Context, stage *types.StageRun) error {
	inputManifest, outputManifest, metrics, err := stageJSON(stage)
	if err != nil {
		return err
	}

	query := `
		UPDATE distill_stage_runs
		SET status = ?,
			container_id = ?,
			node_name = ?,
			config_path = ?,
			input_manifest = ?,
			output_manifest = ?,
			metrics = ?,
			log_path = ?,
			retry_count = ?,
			error_message = ?,
			started_at = ?,
			finished_at = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query,
		stage.Status,
		stage.ContainerID,
		stage.NodeName,
		stage.ConfigPath,
		inputManifest,
		outputManifest,
		metrics,
		stage.LogPath,
		stage.RetryCount,
		stage.ErrorMessage,
		stage.StartedAt,
		stage.FinishedAt,
		stage.ID,
	)
	if err != nil {
		return fmt.Errorf("update stage run failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("stage run not found: %s", stage.ID)
	}
	return nil
}

func (r *stageRepo) UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	query := `
		UPDATE distill_stage_runs
		SET status = ?,
			error_message = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("update stage status failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("stage run not found: %s", id)
	}
	return nil
}

func (r *stageRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.sql.ExecContext(ctx, `DELETE FROM distill_stage_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete stage run failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("stage run not found: %s", id)
	}
	return nil
}

func stageJSON(stage *types.StageRun) ([]byte, []byte, []byte, error) {
	inputManifest, err := jsonBytes(stage.InputManifest)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal input_manifest failed: %w", err)
	}
	outputManifest, err := jsonBytes(stage.OutputManifest)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal output_manifest failed: %w", err)
	}
	metrics, err := jsonBytes(stage.Metrics)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal metrics failed: %w", err)
	}
	return inputManifest, outputManifest, metrics, nil
}

func scanStageJSON(stage *types.StageRun, inputManifest, outputManifest, metrics []byte) error {
	if err := unmarshalJSON(inputManifest, &stage.InputManifest); err != nil {
		return fmt.Errorf("unmarshal input_manifest failed: %w", err)
	}
	if err := unmarshalJSON(outputManifest, &stage.OutputManifest); err != nil {
		return fmt.Errorf("unmarshal output_manifest failed: %w", err)
	}
	if err := unmarshalJSON(metrics, &stage.Metrics); err != nil {
		return fmt.Errorf("unmarshal metrics failed: %w", err)
	}
	return nil
}
