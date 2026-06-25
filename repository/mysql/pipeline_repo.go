package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/google/uuid"
)

type PipelineRepository interface {
	Create(ctx context.Context, pipeline *types.PipelineRun) error
	GetByID(ctx context.Context, id string) (*types.PipelineRun, error)
	List(ctx context.Context, uid int, projectID string, limit, offset int) ([]*types.PipelineRun, error)
	Update(ctx context.Context, pipeline *types.PipelineRun) error
	UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	Delete(ctx context.Context, id string) error
	CountByProject(ctx context.Context, uid int, projectID string) (int, error)
}

type pipelineRepo struct {
	db *DB
}

func NewPipelineRepository(db *DB) PipelineRepository {
	return &pipelineRepo{db: db}
}

func (r *pipelineRepo) Create(ctx context.Context, pipeline *types.PipelineRun) error {
	if pipeline.ID == "" {
		pipeline.ID = uuid.NewString()
	}
	now := time.Now()
	pipeline.CreatedAt = now
	pipeline.UpdatedAt = now

	trainingConfig, err := jsonBytes(pipeline.TrainingConfig)
	if err != nil {
		return fmt.Errorf("marshal training_config failed: %w", err)
	}
	resourceRequest, err := jsonBytes(pipeline.ResourceRequest)
	if err != nil {
		return fmt.Errorf("marshal resource_request failed: %w", err)
	}

	query := `
		INSERT INTO distill_pipeline_runs (
			id, uid, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := r.db.sql.ExecContext(ctx, query,
		pipeline.ID,
		pipeline.UID,
		pipeline.ProjectID,
		pipeline.DatasetID,
		pipeline.Status,
		pipeline.CurrentStage,
		pipeline.TriggerMode,
		trainingConfig,
		resourceRequest,
		pipeline.CreatedAt,
		pipeline.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create pipeline run failed: %w", err)
	}
	return nil
}

func (r *pipelineRepo) GetByID(ctx context.Context, id string) (*types.PipelineRun, error) {
	query := `
		SELECT id, uid, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request, error_message,
			created_at, started_at, finished_at, updated_at
		FROM distill_pipeline_runs
		WHERE id = ?
	`

	var pipeline types.PipelineRun
	var trainingConfig, resourceRequest []byte
	var errorMessage sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := r.db.sql.QueryRowContext(ctx, query, id).Scan(
		&pipeline.ID,
		&pipeline.UID,
		&pipeline.ProjectID,
		&pipeline.DatasetID,
		&pipeline.Status,
		&pipeline.CurrentStage,
		&pipeline.TriggerMode,
		&trainingConfig,
		&resourceRequest,
		&errorMessage,
		&pipeline.CreatedAt,
		&startedAt,
		&finishedAt,
		&pipeline.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pipeline run not found: %s", id)
		}
		return nil, fmt.Errorf("query pipeline run failed: %w", err)
	}
	if err := scanPipelineJSON(&pipeline, trainingConfig, resourceRequest); err != nil {
		return nil, err
	}
	pipeline.ErrorMessage = nullStringValue(errorMessage)
	pipeline.StartedAt = nullTimePtr(startedAt)
	pipeline.FinishedAt = nullTimePtr(finishedAt)
	return &pipeline, nil
}

func (r *pipelineRepo) List(ctx context.Context, uid int, projectID string, limit, offset int) ([]*types.PipelineRun, error) {
	query := `
		SELECT id, uid, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request, error_message,
			created_at, started_at, finished_at, updated_at
		FROM distill_pipeline_runs
		WHERE uid = ? AND project_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.sql.QueryContext(ctx, query, uid, projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query pipeline run list failed: %w", err)
	}
	defer rows.Close()

	var pipelines []*types.PipelineRun
	for rows.Next() {
		var pipeline types.PipelineRun
		var trainingConfig, resourceRequest []byte
		var errorMessage sql.NullString
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&pipeline.ID,
			&pipeline.UID,
			&pipeline.ProjectID,
			&pipeline.DatasetID,
			&pipeline.Status,
			&pipeline.CurrentStage,
			&pipeline.TriggerMode,
			&trainingConfig,
			&resourceRequest,
			&errorMessage,
			&pipeline.CreatedAt,
			&startedAt,
			&finishedAt,
			&pipeline.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan pipeline row failed: %w", err)
		}
		if err := scanPipelineJSON(&pipeline, trainingConfig, resourceRequest); err != nil {
			return nil, err
		}
		pipeline.ErrorMessage = nullStringValue(errorMessage)
		pipeline.StartedAt = nullTimePtr(startedAt)
		pipeline.FinishedAt = nullTimePtr(finishedAt)
		pipelines = append(pipelines, &pipeline)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pipeline rows failed: %w", err)
	}
	return pipelines, nil
}

func (r *pipelineRepo) Update(ctx context.Context, pipeline *types.PipelineRun) error {
	trainingConfig, err := jsonBytes(pipeline.TrainingConfig)
	if err != nil {
		return fmt.Errorf("marshal training_config failed: %w", err)
	}
	resourceRequest, err := jsonBytes(pipeline.ResourceRequest)
	if err != nil {
		return fmt.Errorf("marshal resource_request failed: %w", err)
	}

	query := `
		UPDATE distill_pipeline_runs
		SET uid = ?,
			status = ?,
			current_stage = ?,
			training_config = ?,
			resource_request = ?,
			error_message = ?,
			started_at = ?,
			finished_at = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query,
		pipeline.UID,
		pipeline.Status,
		pipeline.CurrentStage,
		trainingConfig,
		resourceRequest,
		pipeline.ErrorMessage,
		pipeline.StartedAt,
		pipeline.FinishedAt,
		pipeline.ID,
	)
	if err != nil {
		return fmt.Errorf("update pipeline run failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("pipeline run not found: %s", pipeline.ID)
	}
	return nil
}

func (r *pipelineRepo) UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	query := `
		UPDATE distill_pipeline_runs
		SET status = ?,
			error_message = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("update pipeline status failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("pipeline run not found: %s", id)
	}
	return nil
}

func (r *pipelineRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.sql.ExecContext(ctx, `DELETE FROM distill_pipeline_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete pipeline run failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("pipeline run not found: %s", id)
	}
	return nil
}

func (r *pipelineRepo) CountByProject(ctx context.Context, uid int, projectID string) (int, error) {
	var count int
	err := r.db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM distill_pipeline_runs WHERE uid = ? AND project_id = ?`, uid, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pipeline runs failed: %w", err)
	}
	return count, nil
}

func scanPipelineJSON(pipeline *types.PipelineRun, trainingConfig, resourceRequest []byte) error {
	if err := unmarshalJSON(trainingConfig, &pipeline.TrainingConfig); err != nil {
		return fmt.Errorf("unmarshal training_config failed: %w", err)
	}
	if err := unmarshalJSON(resourceRequest, &pipeline.ResourceRequest); err != nil {
		return fmt.Errorf("unmarshal resource_request failed: %w", err)
	}
	return nil
}
