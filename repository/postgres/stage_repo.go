package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/jackc/pgx/v5"
)

// StageRepository 阶段仓库接口
type StageRepository interface {
	Create(ctx context.Context, stage *types.StageRun) error
	GetByID(ctx context.Context, id string) (*types.StageRun, error)
	ListByPipeline(ctx context.Context, pipelineID string) ([]*types.StageRun, error)
	Update(ctx context.Context, stage *types.StageRun) error
	UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error
	Delete(ctx context.Context, id string) error
}

// stageRepo 阶段仓库实现
type stageRepo struct {
	db *DB
}

// NewStageRepository 创建阶段仓库
func NewStageRepository(db *DB) StageRepository {
	return &stageRepo{db: db}
}

// Create 创建阶段运行
func (r *stageRepo) Create(ctx context.Context, stage *types.StageRun) error {
	// 序列化 JSON 字段
	inputManifest, err := json.Marshal(stage.InputManifest)
	if err != nil {
		return fmt.Errorf("序列化输入清单失败: %w", err)
	}

	outputManifest, err := json.Marshal(stage.OutputManifest)
	if err != nil {
		return fmt.Errorf("序列化输出清单失败: %w", err)
	}

	metrics, err := json.Marshal(stage.Metrics)
	if err != nil {
		return fmt.Errorf("序列化指标失败: %w", err)
	}

	query := `
		INSERT INTO stage_runs (
			id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id, created_at, updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
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
	).Scan(&stage.ID, &stage.CreatedAt, &stage.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建阶段运行失败: %w", err)
	}

	return nil
}

// GetByID 根据 ID 获取阶段运行
func (r *stageRepo) GetByID(ctx context.Context, id string) (*types.StageRun, error) {
	query := `
		SELECT id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count, error_message,
			started_at, finished_at, created_at, updated_at
		FROM stage_runs
		WHERE id = $1
	`

	var stage types.StageRun
	var inputManifest, outputManifest, metrics []byte
	var containerID, nodeName, configPath, logPath, errorMessage sql.NullString

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
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
		&stage.StartedAt,
		&stage.FinishedAt,
		&stage.CreatedAt,
		&stage.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("阶段运行不存在: %s", id)
		}
		return nil, fmt.Errorf("查询阶段运行失败: %w", err)
	}

	// 反序列化 JSON 字段
	if len(inputManifest) > 0 {
		if err := json.Unmarshal(inputManifest, &stage.InputManifest); err != nil {
			return nil, fmt.Errorf("反序列化输入清单失败: %w", err)
		}
	}

	if len(outputManifest) > 0 {
		if err := json.Unmarshal(outputManifest, &stage.OutputManifest); err != nil {
			return nil, fmt.Errorf("反序列化输出清单失败: %w", err)
		}
	}

	if len(metrics) > 0 {
		if err := json.Unmarshal(metrics, &stage.Metrics); err != nil {
			return nil, fmt.Errorf("反序列化指标失败: %w", err)
		}
	}

	stage.ContainerID = nullStringValue(containerID)
	stage.NodeName = nullStringValue(nodeName)
	stage.ConfigPath = nullStringValue(configPath)
	stage.LogPath = nullStringValue(logPath)
	stage.ErrorMessage = nullStringValue(errorMessage)

	return &stage, nil
}

// ListByPipeline 列出流水线的所有阶段
func (r *stageRepo) ListByPipeline(ctx context.Context, pipelineID string) ([]*types.StageRun, error) {
	query := `
		SELECT id, pipeline_run_id, stage_type, stage_order, status,
			container_id, node_name, config_path,
			input_manifest, output_manifest, metrics,
			log_path, retry_count, error_message,
			started_at, finished_at, created_at, updated_at
		FROM stage_runs
		WHERE pipeline_run_id = $1
		ORDER BY stage_order ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("查询阶段运行列表失败: %w", err)
	}
	defer rows.Close()

	var stages []*types.StageRun
	for rows.Next() {
		var stage types.StageRun
		var inputManifest, outputManifest, metrics []byte
		var containerID, nodeName, configPath, logPath, errorMessage sql.NullString

		err := rows.Scan(
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
			&stage.StartedAt,
			&stage.FinishedAt,
			&stage.CreatedAt,
			&stage.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描阶段运行数据失败: %w", err)
		}

		// 反序列化 JSON 字段
		if len(inputManifest) > 0 {
			if err := json.Unmarshal(inputManifest, &stage.InputManifest); err != nil {
				return nil, fmt.Errorf("反序列化输入清单失败: %w", err)
			}
		}

		if len(outputManifest) > 0 {
			if err := json.Unmarshal(outputManifest, &stage.OutputManifest); err != nil {
				return nil, fmt.Errorf("反序列化输出清单失败: %w", err)
			}
		}

		if len(metrics) > 0 {
			if err := json.Unmarshal(metrics, &stage.Metrics); err != nil {
				return nil, fmt.Errorf("反序列化指标失败: %w", err)
			}
		}

		stage.ContainerID = nullStringValue(containerID)
		stage.NodeName = nullStringValue(nodeName)
		stage.ConfigPath = nullStringValue(configPath)
		stage.LogPath = nullStringValue(logPath)
		stage.ErrorMessage = nullStringValue(errorMessage)

		stages = append(stages, &stage)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历阶段运行数据失败: %w", err)
	}

	return stages, nil
}

// Update 更新阶段运行
func (r *stageRepo) Update(ctx context.Context, stage *types.StageRun) error {
	// 序列化 JSON 字段
	inputManifest, err := json.Marshal(stage.InputManifest)
	if err != nil {
		return fmt.Errorf("序列化输入清单失败: %w", err)
	}

	outputManifest, err := json.Marshal(stage.OutputManifest)
	if err != nil {
		return fmt.Errorf("序列化输出清单失败: %w", err)
	}

	metrics, err := json.Marshal(stage.Metrics)
	if err != nil {
		return fmt.Errorf("序列化指标失败: %w", err)
	}

	query := `
		UPDATE stage_runs
		SET status = $1,
			container_id = $2,
			node_name = $3,
			config_path = $4,
			input_manifest = $5,
			output_manifest = $6,
			metrics = $7,
			log_path = $8,
			retry_count = $9,
			error_message = $10,
			started_at = $11,
			finished_at = $12
		WHERE id = $13
	`

	result, err := r.db.Pool.Exec(ctx, query,
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
		return fmt.Errorf("更新阶段运行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("阶段运行不存在: %s", stage.ID)
	}

	return nil
}

// UpdateStatus 更新阶段状态
func (r *stageRepo) UpdateStatus(ctx context.Context, id string, status types.PipelineStatus, errorMsg string) error {
	query := `
		UPDATE stage_runs
		SET status = $1,
			error_message = $2
		WHERE id = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, status, errorMsg, id)
	if err != nil {
		return fmt.Errorf("更新阶段状态失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("阶段运行不存在: %s", id)
	}

	return nil
}

// Delete 删除阶段运行
func (r *stageRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM stage_runs WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除阶段运行失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("阶段运行不存在: %s", id)
	}

	return nil
}
