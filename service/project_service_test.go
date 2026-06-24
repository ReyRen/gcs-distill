package service

import (
	"context"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestProjectServiceResolvesLocalModelsByID(t *testing.T) {
	repo := &fakeProjectRepo{}
	models := &fakeModelService{
		teacher: &LocalModel{
			ID:   "teacher-qwen",
			Name: "Qwen2.5-7B-Instruct",
			Path: "/storage-root-jfs/train-base-models/Qwen2.5-7B-Instruct",
		},
		student: &LocalModel{
			ID:   "student-qwen",
			Name: "Qwen2.5-0.5B-Instruct",
			Path: "/storage-root-jfs/train-base-models/Qwen2.5-0.5B-Instruct",
		},
	}
	svc := NewProjectService(repo, models)

	project := &types.Project{
		Name: "distill-test",
		TeacherModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelID:      "teacher-qwen",
		},
		StudentModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
			ModelID:      "student-qwen",
		},
	}
	if err := svc.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if project.TeacherModelConfig.ModelPath != models.teacher.Path {
		t.Fatalf("teacher model_path = %q, want %q", project.TeacherModelConfig.ModelPath, models.teacher.Path)
	}
	if project.TeacherModelConfig.ModelName != models.teacher.Name {
		t.Fatalf("teacher model_name = %q, want %q", project.TeacherModelConfig.ModelName, models.teacher.Name)
	}
	if project.StudentModelConfig.ModelPath != models.student.Path {
		t.Fatalf("student model_path = %q, want %q", project.StudentModelConfig.ModelPath, models.student.Path)
	}
	if project.StudentModelConfig.ModelName != models.student.Name {
		t.Fatalf("student model_name = %q, want %q", project.StudentModelConfig.ModelName, models.student.Name)
	}
}

func TestProjectServiceRequiresLocalModelIDOrPath(t *testing.T) {
	svc := NewProjectService(&fakeProjectRepo{}, &fakeModelService{})

	err := svc.CreateProject(context.Background(), &types.Project{
		Name: "distill-test",
		TeacherModelConfig: types.ModelConfig{
			ProviderType: types.ProviderAPI,
			ModelName:    "qwen-max",
		},
		StudentModelConfig: types.ModelConfig{
			ProviderType: types.ProviderLocal,
		},
	})
	if err == nil {
		t.Fatal("CreateProject() error = nil, want local model_id validation error")
	}
}

type fakeProjectRepo struct {
	project *types.Project
}

func (r *fakeProjectRepo) Create(_ context.Context, project *types.Project) error {
	r.project = project
	return nil
}

func (r *fakeProjectRepo) GetByID(_ context.Context, _ string) (*types.Project, error) {
	if r.project != nil {
		return r.project, nil
	}
	return &types.Project{ID: "project-1", Name: "distill-test"}, nil
}

func (r *fakeProjectRepo) List(_ context.Context, _, _ int) ([]*types.Project, error) {
	return nil, nil
}

func (r *fakeProjectRepo) Update(_ context.Context, project *types.Project) error {
	r.project = project
	return nil
}

func (r *fakeProjectRepo) Delete(_ context.Context, _ string) error {
	r.project = nil
	return nil
}

func (r *fakeProjectRepo) Count(_ context.Context) (int, error) {
	return 0, nil
}

type fakeModelService struct {
	teacher *LocalModel
	student *LocalModel
}

func (s *fakeModelService) ListTeacherModels(context.Context) ([]*TeacherModel, error) {
	return []*TeacherModel{s.teacher}, nil
}

func (s *fakeModelService) GetTeacherModel(_ context.Context, _ string) (*TeacherModel, error) {
	return s.teacher, nil
}

func (s *fakeModelService) ListStudentModels(context.Context) ([]*StudentModel, error) {
	return []*StudentModel{s.student}, nil
}

func (s *fakeModelService) GetStudentModel(_ context.Context, _ string) (*StudentModel, error) {
	return s.student, nil
}

func (s *fakeModelService) ValidateLocalModel(context.Context, string) error {
	return nil
}

func (s *fakeModelService) ValidateStudentModel(context.Context, string) error {
	return nil
}
