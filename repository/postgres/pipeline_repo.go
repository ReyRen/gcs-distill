package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/jackc/pgx/v5"
)

// PipelineRepository 流水线仓库接口
type PipelineRepository interface {
	Create(ctx context.Context, pipeline *types.PipelineRun) error
	GetByID(ctx context.Context, id string) (*types.PipelineRun, error)
	List(ctx context.Context, projectID string, limit, offset int) ([]*types.PipelineRun, error)
	Update(ctx context.Context, pipeline *types.PipelineRun) error
	UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	Delete(ctx context.Context, id string) error
	CountByProject(ctx context.Context, projectID string) (int, error)
}

// pipelineRepo 流水线仓库实现
type pipelineRepo struct {
	db *DB
}

// NewPipelineRepository 创建流水线仓库
func NewPipelineRepository(db *DB) PipelineRepository {
	return &pipelineRepo{db: db}
}

// Create 创建流水线运行
func (r *pipelineRepo) Create(ctx context.Context, pipeline *types.PipelineRun) error {
	// 序列化 JSON 字段
	trainingConfig, err := json.Marshal(pipeline.TrainingConfig)
	if err != nil {
		return fmt.Errorf("序列化训练配置失败: %w", err)
	}

	resourceRequest, err := json.Marshal(pipeline.ResourceRequest)
	if err != nil {
		return fmt.Errorf("序列化资源请求失败: %w", err)
	}

	query := `
		INSERT INTO pipeline_runs (
			id, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7
		) RETURNING id, created_at, updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		pipeline.ProjectID,
		pipeline.DatasetID,
		pipeline.Status,
		pipeline.CurrentStage,
		pipeline.TriggerMode,
		trainingConfig,
		resourceRequest,
	).Scan(&pipeline.ID, &pipeline.CreatedAt, &pipeline.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建流水线运行失败: %w", err)
	}

	return nil
}

// GetByID 根据 ID 获取流水线运行
func (r *pipelineRepo) GetByID(ctx context.Context, id string) (*types.PipelineRun, error) {
	query := `
		SELECT id, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request, error_message,
			created_at, started_at, finished_at, updated_at
		FROM pipeline_runs
		WHERE id = $1
	`

	var pipeline types.PipelineRun
	var trainingConfig, resourceRequest []byte
	var errorMessage sql.NullString

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&pipeline.ID,
		&pipeline.ProjectID,
		&pipeline.DatasetID,
		&pipeline.Status,
		&pipeline.CurrentStage,
		&pipeline.TriggerMode,
		&trainingConfig,
		&resourceRequest,
		&errorMessage,
		&pipeline.CreatedAt,
		&pipeline.StartedAt,
		&pipeline.FinishedAt,
		&pipeline.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("流水线运行不存在: %s", id)
		}
		return nil, fmt.Errorf("查询流水线运行失败: %w", err)
	}

	// 反序列化 JSON 字段
	if err := json.Unmarshal(trainingConfig, &pipeline.TrainingConfig); err != nil {
		return nil, fmt.Errorf("反序列化训练配置失败: %w", err)
	}

	if err := json.Unmarshal(resourceRequest, &pipeline.ResourceRequest); err != nil {
		return nil, fmt.Errorf("反序列化资源请求失败: %w", err)
	}

	pipeline.ErrorMessage = nullStringValue(errorMessage)

	return &pipeline, nil
}

// List 列出流水线运行
func (r *pipelineRepo) List(ctx context.Context, projectID string, limit, offset int) ([]*types.PipelineRun, error) {
	query := `
		SELECT id, project_id, dataset_id, status, current_stage, trigger_mode,
			training_config, resource_request, error_message,
			created_at, started_at, finished_at, updated_at
		FROM pipeline_runs
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询流水线运行列表失败: %w", err)
	}
	defer rows.Close()

	var pipelines []*types.PipelineRun
	for rows.Next() {
		var pipeline types.PipelineRun
		var trainingConfig, resourceRequest []byte
		var errorMessage sql.NullString

		err := rows.Scan(
			&pipeline.ID,
			&pipeline.ProjectID,
			&pipeline.DatasetID,
			&pipeline.Status,
			&pipeline.CurrentStage,
			&pipeline.TriggerMode,
			&trainingConfig,
			&resourceRequest,
			&errorMessage,
			&pipeline.CreatedAt,
			&pipeline.StartedAt,
			&pipeline.FinishedAt,
			&pipeline.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描流水线运行数据失败: %w", err)
		}

		// 反序列化 JSON 字段
		if err := json.Unmarshal(trainingConfig, &pipeline.TrainingConfig); err != nil {
			return nil, fmt.Errorf("反序列化训练配置失败: %w", err)
		}

		if err := json.Unmarshal(resourceRequest, &pipeline.ResourceRequest); err != nil {
			return nil, fmt.Errorf("反序列化资源请求失败: %w", err)
		}

		pipeline.ErrorMessage = nullStringValue(errorMessage)

		pipelines = append(pipelines, &pipeline)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历流水线运行数据失败: %w", err)
	}

	return pipelines, nil
}

// Update 更新流水线运行
func (r *pipelineRepo) Update(ctx context.Context, pipeline *types.PipelineRun) error {
	// 序列化 JSON 字段
	trainingConfig, err := json.Marshal(pipeline.TrainingConfig)
	if err != nil {
		return fmt.Errorf("序列化训练配置失败: %w", err)
	}

	resourceRequest, err := json.Marshal(pipeline.ResourceRequest)
	if err != nil {
		return fmt.Errorf("序列化资源请求失败: %w", err)
	}

	query := `
		UPDATE pipeline_runs
		SET status = $1,
			current_stage = $2,
			training_config = $3,
			resource_request = $4,
			error_message = $5,
			started_at = $6,
			finished_at = $7
		WHERE id = $8
	`

	result, err := r.db.Pool.Exec(ctx, query,
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
		return fmt.Errorf("更新流水线运行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("流水线运行不存在: %s", pipeline.ID)
	}

	return nil
}

// UpdateStatus 更新流水线状态
func (r *pipelineRepo) UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	query := `
		UPDATE pipeline_runs
		SET status = $1,
			error_message = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("更新流水线状态失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("流水线运行不存在: %s", id)
	}

	return nil
}

// Delete 删除流水线运行
func (r *pipelineRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pipeline_runs WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除流水线运行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("流水线运行不存在: %s", id)
	}

	return nil
}

// CountByProject 统计项目的流水线运行数量
func (r *pipelineRepo) CountByProject(ctx context.Context, projectID string) (int, error) {
	query := `SELECT COUNT(*) FROM pipeline_runs WHERE project_id = $1`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计流水线运行数量失败: %w", err)
	}

	return count, nil
}
