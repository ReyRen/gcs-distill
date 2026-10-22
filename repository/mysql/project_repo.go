package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/google/uuid"
)

type ProjectRepository interface {
	Create(ctx context.Context, project *types.Project) error
	GetByID(ctx context.Context, id string) (*types.Project, error)
	List(ctx context.Context, uid, limit, offset int, filters ...types.ListFilter) ([]*types.Project, error)
	Update(ctx context.Context, project *types.Project) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, uid int, filters ...types.ListFilter) (int, error)
}

type projectRepo struct {
	db *DB
}

func NewProjectRepository(db *DB) ProjectRepository {
	return &projectRepo{db: db}
}

func (r *projectRepo) Create(ctx context.Context, project *types.Project) error {
	if project.ID == "" {
		project.ID = uuid.NewString()
	}
	now := time.Now()
	project.CreatedAt = now
	project.UpdatedAt = now

	teacherConfig, err := jsonBytes(project.TeacherModelConfig)
	if err != nil {
		return fmt.Errorf("marshal teacher_model_config failed: %w", err)
	}

	query := `
		INSERT INTO distill_projects (
			id, uid, name, description, business_scenario,
			teacher_model_config,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := r.db.sql.ExecContext(ctx, query,
		project.ID,
		project.UID,
		project.Name,
		project.Description,
		project.BusinessScenario,
		teacherConfig,
		project.CreatedAt,
		project.UpdatedAt,
	); err != nil {
		return fmt.Errorf("create project failed: %w", err)
	}
	return nil
}

func (r *projectRepo) GetByID(ctx context.Context, id string) (*types.Project, error) {
	query := `
		SELECT id, uid, name, description, business_scenario,
			teacher_model_config,
			created_at, updated_at
		FROM distill_projects
		WHERE id = ?
	`

	var project types.Project
	var teacherConfig []byte
	var description, businessScenario sql.NullString
	err := r.db.sql.QueryRowContext(ctx, query, id).Scan(
		&project.ID,
		&project.UID,
		&project.Name,
		&description,
		&businessScenario,
		&teacherConfig,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, fmt.Errorf("query project failed: %w", err)
	}
	if err := scanProjectJSON(&project, teacherConfig); err != nil {
		return nil, err
	}
	project.Description = nullStringValue(description)
	project.BusinessScenario = nullStringValue(businessScenario)
	return &project, nil
}

func (r *projectRepo) List(ctx context.Context, uid, limit, offset int, filters ...types.ListFilter) ([]*types.Project, error) {
	where, args := listConditions(uid, "", "project", filters)
	args = append(args, limit, offset)
	query := `
		SELECT id, uid, name, description, business_scenario,
			teacher_model_config,
			created_at, updated_at
		FROM distill_projects
		WHERE ` + where + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query project list failed: %w", err)
	}
	defer rows.Close()

	var projects []*types.Project
	for rows.Next() {
		var project types.Project
		var teacherConfig []byte
		var description, businessScenario sql.NullString
		if err := rows.Scan(
			&project.ID,
			&project.UID,
			&project.Name,
			&description,
			&businessScenario,
			&teacherConfig,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project row failed: %w", err)
		}
		if err := scanProjectJSON(&project, teacherConfig); err != nil {
			return nil, err
		}
		project.Description = nullStringValue(description)
		project.BusinessScenario = nullStringValue(businessScenario)
		projects = append(projects, &project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows failed: %w", err)
	}
	return projects, nil
}

func (r *projectRepo) Update(ctx context.Context, project *types.Project) error {
	teacherConfig, err := jsonBytes(project.TeacherModelConfig)
	if err != nil {
		return fmt.Errorf("marshal teacher_model_config failed: %w", err)
	}

	query := `
		UPDATE distill_projects
		SET uid = ?,
			name = ?,
			description = ?,
			business_scenario = ?,
			teacher_model_config = ?
		WHERE id = ?
	`
	result, err := r.db.sql.ExecContext(ctx, query,
		project.UID,
		project.Name,
		project.Description,
		project.BusinessScenario,
		teacherConfig,
		project.ID,
	)
	if err != nil {
		return fmt.Errorf("update project failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("project not found: %s", project.ID)
	}
	return nil
}

func (r *projectRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.sql.ExecContext(ctx, `DELETE FROM distill_projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project failed: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

func (r *projectRepo) Count(ctx context.Context, uid int, filters ...types.ListFilter) (int, error) {
	var count int
	where, args := listConditions(uid, "", "project", filters)
	if err := r.db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM distill_projects WHERE `+where, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count projects failed: %w", err)
	}
	return count, nil
}

func scanProjectJSON(project *types.Project, teacherConfig []byte) error {
	if err := unmarshalJSON(teacherConfig, &project.TeacherModelConfig); err != nil {
		return fmt.Errorf("unmarshal teacher_model_config failed: %w", err)
	}
	return nil
}
