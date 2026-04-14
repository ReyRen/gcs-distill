package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/jackc/pgx/v5"
)

// DatasetRepository 数据集仓库接口
type DatasetRepository interface {
	Create(ctx context.Context, dataset *types.Dataset) error
	GetByID(ctx context.Context, id string) (*types.Dataset, error)
	ListByProject(ctx context.Context, projectID string, limit, offset int) ([]*types.Dataset, error)
	Update(ctx context.Context, dataset *types.Dataset) error
	Delete(ctx context.Context, id string) error
	CountByProject(ctx context.Context, projectID string) (int, error)
}

// datasetRepo 数据集仓库实现
type datasetRepo struct {
	db *DB
}

// NewDatasetRepository 创建数据集仓库
func NewDatasetRepository(db *DB) DatasetRepository {
	return &datasetRepo{db: db}
}

// Create 创建数据集
func (r *datasetRepo) Create(ctx context.Context, dataset *types.Dataset) error {
	query := `
		INSERT INTO datasets (
			id, project_id, name, description, source_type, file_path, record_count
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		dataset.ID,
		dataset.ProjectID,
		dataset.Name,
		dataset.Description,
		dataset.SourceType,
		dataset.FilePath,
		dataset.RecordCount,
	).Scan(&dataset.CreatedAt)

	if err != nil {
		return fmt.Errorf("创建数据集失败: %w", err)
	}

	return nil
}

// GetByID 根据 ID 获取数据集
func (r *datasetRepo) GetByID(ctx context.Context, id string) (*types.Dataset, error) {
	query := `
		SELECT id, project_id, name, description, source_type, file_path, record_count, created_at
		FROM datasets
		WHERE id = $1
	`

	var dataset types.Dataset
	var description sql.NullString
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&dataset.ID,
		&dataset.ProjectID,
		&dataset.Name,
		&description,
		&dataset.SourceType,
		&dataset.FilePath,
		&dataset.RecordCount,
		&dataset.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("数据集不存在: %s", id)
		}
		return nil, fmt.Errorf("查询数据集失败: %w", err)
	}

	dataset.Description = nullStringValue(description)

	return &dataset, nil
}

// ListByProject 列出项目的数据集
func (r *datasetRepo) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]*types.Dataset, error) {
	query := `
		SELECT id, project_id, name, description, source_type, file_path, record_count, created_at
		FROM datasets
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询数据集列表失败: %w", err)
	}
	defer rows.Close()

	var datasets []*types.Dataset
	for rows.Next() {
		var dataset types.Dataset
		var description sql.NullString
		err := rows.Scan(
			&dataset.ID,
			&dataset.ProjectID,
			&dataset.Name,
			&description,
			&dataset.SourceType,
			&dataset.FilePath,
			&dataset.RecordCount,
			&dataset.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描数据集数据失败: %w", err)
		}

		dataset.Description = nullStringValue(description)

		datasets = append(datasets, &dataset)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历数据集数据失败: %w", err)
	}

	return datasets, nil
}

// Update 更新数据集
func (r *datasetRepo) Update(ctx context.Context, dataset *types.Dataset) error {
	query := `
		UPDATE datasets
		SET name = $1,
			description = $2,
			record_count = $3
		WHERE id = $4
	`

	result, err := r.db.Pool.Exec(ctx, query,
		dataset.Name,
		dataset.Description,
		dataset.RecordCount,
		dataset.ID,
	)

	if err != nil {
		return fmt.Errorf("更新数据集失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("数据集不存在: %s", dataset.ID)
	}

	return nil
}

// Delete 删除数据集
func (r *datasetRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM datasets WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除数据集失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("数据集不存在: %s", id)
	}

	return nil
}

// CountByProject 统计项目的数据集数量
func (r *datasetRepo) CountByProject(ctx context.Context, projectID string) (int, error) {
	query := `SELECT COUNT(*) FROM datasets WHERE project_id = $1`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计数据集数量失败: %w", err)
	}

	return count, nil
}
