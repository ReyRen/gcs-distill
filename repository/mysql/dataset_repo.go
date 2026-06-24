package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/google/uuid"
)

type DatasetRepository interface {
	Create(ctx context.Context, dataset *types.Dataset) error
	GetByID(ctx context.Context, id string) (*types.Dataset, error)
	List(ctx context.Context, limit, offset int) ([]*types.Dataset, error)
	Update(ctx context.Context, dataset *types.Dataset) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
}

type datasetRepo struct {
	db *DB
}

func NewDatasetRepository(db *DB) DatasetRepository {
	return &datasetRepo{db: db}
}

func (r *datasetRepo) Create(ctx context.Context, dataset *types.Dataset) error {
	if dataset.ID == "" {
		dataset.ID = uuid.NewString()
	}
	dataset.CreatedAt = time.Now()

	query := `
		INSERT INTO distill_datasets (
			id, name, description, source_type, file_path, record_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := r.db.sql.ExecContext(ctx, query,
		dataset.ID,
		dataset.Name,
		dataset.Description,
		dataset.SourceType,
		dataset.FilePath,
		dataset.RecordCount,
		dataset.CreatedAt,
	); err != nil {
		return fmt.Errorf("create dataset failed: %w", err)
	}
	return nil
}

func (r *datasetRepo) GetByID(ctx context.Context, id string) (*types.Dataset, error) {
	query := `
		SELECT id, name, description, source_type, file_path, record_count, created_at
		FROM distill_datasets
		WHERE id = ?
	`

	var dataset types.Dataset
	var description sql.NullString
	err := r.db.sql.QueryRowContext(ctx, query, id).Scan(
		&dataset.ID,
		&dataset.Name,
		&description,
		&dataset.SourceType,
		&dataset.FilePath,
		&dataset.RecordCount,
		&dataset.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("dataset not found: %s", id)
		}
		return nil, fmt.Errorf("query dataset failed: %w", err)
	}
	dataset.Description = nullStringValue(description)
	return &dataset, nil
}

func (r *datasetRepo) List(ctx context.Context, limit, offset int) ([]*types.Dataset, error) {
	query := `
		SELECT id, name, description, source_type, file_path, record_count, created_at
		FROM distill_datasets
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.sql.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query dataset list failed: %w", err)
	}
	defer rows.Close()

	var datasets []*types.Dataset
	for rows.Next() {
		var dataset types.Dataset
		var description sql.NullString
		if err := rows.Scan(
			&dataset.ID,
			&dataset.Name,
			&description,
			&dataset.SourceType,
			&dataset.FilePath,
			&dataset.RecordCount,
			&dataset.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan dataset row failed: %w", err)
		}
		dataset.Description = nullStringValue(description)
		datasets = append(datasets, &dataset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dataset rows failed: %w", err)
	}
	return datasets, nil
}

func (r *datasetRepo) Update(ctx context.Context, dataset *types.Dataset) error {
	query := `
		UPDATE distill_datasets
		SET name = ?,
			description = ?,
			record_count = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query,
		dataset.Name,
		dataset.Description,
		dataset.RecordCount,
		dataset.ID,
	)
	if err != nil {
		return fmt.Errorf("update dataset failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("dataset not found: %s", dataset.ID)
	}
	return nil
}

func (r *datasetRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.sql.ExecContext(ctx, `DELETE FROM distill_datasets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete dataset failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("dataset not found: %s", id)
	}
	return nil
}

func (r *datasetRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM distill_datasets`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count datasets failed: %w", err)
	}
	return count, nil
}
