package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/jackc/pgx/v5"
)

// ProjectRepository 项目仓库接口
type ProjectRepository interface {
	Create(ctx context.Context, project *types.Project) error
	GetByID(ctx context.Context, id string) (*types.Project, error)
	List(ctx context.Context, limit, offset int) ([]*types.Project, error)
	Update(ctx context.Context, project *types.Project) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
}

// projectRepo 项目仓库实现
type projectRepo struct {
	db *DB
}

// NewProjectRepository 创建项目仓库
func NewProjectRepository(db *DB) ProjectRepository {
	return &projectRepo{db: db}
}

// Create 创建项目
func (r *projectRepo) Create(ctx context.Context, project *types.Project) error {
	// 序列化 JSON 字段
	teacherConfig, err := json.Marshal(project.TeacherModelConfig)
	if err != nil {
		return fmt.Errorf("序列化教师模型配置失败: %w", err)
	}

	studentConfig, err := json.Marshal(project.StudentModelConfig)
	if err != nil {
		return fmt.Errorf("序列化学生模型配置失败: %w", err)
	}

	var evalConfig []byte
	if project.EvaluationConfig != nil {
		evalConfig, err = json.Marshal(project.EvaluationConfig)
		if err != nil {
			return fmt.Errorf("序列化评估配置失败: %w", err)
		}
	}

	query := `
		INSERT INTO distill_projects (
			id, name, description, business_scenario,
			teacher_model_config, student_model_config, evaluation_config
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4, $5, $6
		) RETURNING id, created_at, updated_at
	`

	err = r.db.Pool.QueryRow(ctx, query,
		project.Name,
		project.Description,
		project.BusinessScenario,
		teacherConfig,
		studentConfig,
		evalConfig,
	).Scan(&project.ID, &project.CreatedAt, &project.UpdatedAt)

	if err != nil {
		return fmt.Errorf("创建项目失败: %w", err)
	}

	return nil
}

// GetByID 根据 ID 获取项目
func (r *projectRepo) GetByID(ctx context.Context, id string) (*types.Project, error) {
	query := `
		SELECT id, name, description, business_scenario,
			teacher_model_config, student_model_config, evaluation_config,
			created_at, updated_at
		FROM distill_projects
		WHERE id = $1
	`

	var project types.Project
	var teacherConfig, studentConfig, evalConfig []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&project.ID,
		&project.Name,
		&project.Description,
		&project.BusinessScenario,
		&teacherConfig,
		&studentConfig,
		&evalConfig,
		&project.CreatedAt,
		&project.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("项目不存在: %s", id)
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 反序列化 JSON 字段
	if err := json.Unmarshal(teacherConfig, &project.TeacherModelConfig); err != nil {
		return nil, fmt.Errorf("反序列化教师模型配置失败: %w", err)
	}

	if err := json.Unmarshal(studentConfig, &project.StudentModelConfig); err != nil {
		return nil, fmt.Errorf("反序列化学生模型配置失败: %w", err)
	}

	if len(evalConfig) > 0 {
		var ec types.EvaluationConfig
		if err := json.Unmarshal(evalConfig, &ec); err != nil {
			return nil, fmt.Errorf("反序列化评估配置失败: %w", err)
		}
		project.EvaluationConfig = &ec
	}

	return &project, nil
}

// List 列出项目
func (r *projectRepo) List(ctx context.Context, limit, offset int) ([]*types.Project, error) {
	query := `
		SELECT id, name, description, business_scenario,
			teacher_model_config, student_model_config, evaluation_config,
			created_at, updated_at
		FROM distill_projects
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询项目列表失败: %w", err)
	}
	defer rows.Close()

	var projects []*types.Project
	for rows.Next() {
		var project types.Project
		var teacherConfig, studentConfig, evalConfig []byte

		err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.Description,
			&project.BusinessScenario,
			&teacherConfig,
			&studentConfig,
			&evalConfig,
			&project.CreatedAt,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描项目数据失败: %w", err)
		}

		// 反序列化 JSON 字段
		if err := json.Unmarshal(teacherConfig, &project.TeacherModelConfig); err != nil {
			return nil, fmt.Errorf("反序列化教师模型配置失败: %w", err)
		}

		if err := json.Unmarshal(studentConfig, &project.StudentModelConfig); err != nil {
			return nil, fmt.Errorf("反序列化学生模型配置失败: %w", err)
		}

		if len(evalConfig) > 0 {
			var ec types.EvaluationConfig
			if err := json.Unmarshal(evalConfig, &ec); err != nil {
				return nil, fmt.Errorf("反序列化评估配置失败: %w", err)
			}
			project.EvaluationConfig = &ec
		}

		projects = append(projects, &project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历项目数据失败: %w", err)
	}

	return projects, nil
}

// Update 更新项目
func (r *projectRepo) Update(ctx context.Context, project *types.Project) error {
	// 序列化 JSON 字段
	teacherConfig, err := json.Marshal(project.TeacherModelConfig)
	if err != nil {
		return fmt.Errorf("序列化教师模型配置失败: %w", err)
	}

	studentConfig, err := json.Marshal(project.StudentModelConfig)
	if err != nil {
		return fmt.Errorf("序列化学生模型配置失败: %w", err)
	}

	var evalConfig []byte
	if project.EvaluationConfig != nil {
		evalConfig, err = json.Marshal(project.EvaluationConfig)
		if err != nil {
			return fmt.Errorf("序列化评估配置失败: %w", err)
		}
	}

	query := `
		UPDATE distill_projects
		SET name = $1,
			description = $2,
			business_scenario = $3,
			teacher_model_config = $4,
			student_model_config = $5,
			evaluation_config = $6
		WHERE id = $7
	`

	result, err := r.db.Pool.Exec(ctx, query,
		project.Name,
		project.Description,
		project.BusinessScenario,
		teacherConfig,
		studentConfig,
		evalConfig,
		project.ID,
	)

	if err != nil {
		return fmt.Errorf("更新项目失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("项目不存在: %s", project.ID)
	}

	return nil
}

// Delete 删除项目
func (r *projectRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM distill_projects WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("删除项目失败: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("项目不存在: %s", id)
	}

	return nil
}

// Count 统计项目数量
func (r *projectRepo) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM distill_projects`

	var count int
	err := r.db.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计项目数量失败: %w", err)
	}

	return count, nil
}
