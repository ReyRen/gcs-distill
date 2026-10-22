package service

import (
	"context"
	"github.com/ReyRen/gcs-distill/internal/types"
	"testing"
)

func TestProjectDoesNotRequireModels(t *testing.T) {
	svc := NewProjectService(&fakeProjectRepo{})
	if err := svc.CreateProject(context.Background(), &types.Project{UID: 380, Name: "业务项目"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateProject(context.Background(), &types.Project{UID: 380, Name: "业务项目", TeacherModelConfig: types.ModelConfig{ProviderType: types.ProviderLocal, ModelName: "old"}}); err == nil {
		t.Fatal("retired local teacher accepted")
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
	return &types.Project{ID: "project-1", UID: 380, Name: "distill-test"}, nil
}

func (r *fakeProjectRepo) List(_ context.Context, _, _, _ int, _ ...types.ListFilter) ([]*types.Project, error) {
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

func (r *fakeProjectRepo) Count(_ context.Context, _ int, _ ...types.ListFilter) (int, error) {
	return 0, nil
}
