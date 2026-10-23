package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
	mysqlrepo "github.com/ReyRen/gcs-distill/repository/mysql"
)

func project2() *types.Project {
	return &types.Project{ID: "project", UID: 380, TeacherModelConfig: types.ModelConfig{ProviderType: types.ProviderAPI, ModelName: "teacher", Endpoint: "http://teacher/v1", APISecretRef: "private-key"}}
}

func TestAllDistill2RecipesCompileAndKeepManagedPaths(t *testing.T) {
	root := t.TempDir()
	g := NewConfigGenerator(root)
	for _, recipe := range WorkflowRecipes() {
		t.Run(recipe.JobType, func(t *testing.T) {
			p := &types.PipelineRun{UID: 380, ID: "run", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: recipe.JobType,
				Parameters: map[string]interface{}{"dataset": map[string]interface{}{"input_path": "/outside/input", "output_path": "/outside/output"}}}}}
			cfg, err := g.GenerateDistill2Config(project2(), p)
			if err != nil {
				t.Fatal(err)
			}
			dataset := cfg["dataset"].(map[string]interface{})
			if !strings.HasPrefix(dataset["input_path"].(string), root) || !strings.HasPrefix(dataset["output_path"].(string), root) {
				t.Fatal("unmanaged data path", dataset)
			}
			backend := cfg["backend"].(map[string]interface{})
			if backend["type"] != "openai" || backend["base_url"] != "http://teacher/v1" {
				t.Fatal(backend)
			}
			encoded, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "PAI_TOKEN_API_KEY") {
				t.Fatal("unresolved recipe credentials")
			}
		})
	}
}

func TestDistill2DoesNotRequireStudentOrTrainingParameters(t *testing.T) {
	p := &types.PipelineRun{}
	if err := NormalizeDistillation(p, project2()); err != nil {
		t.Fatal(err)
	}
	if len(PipelineStageTypes(p)) != 4 {
		t.Fatal(PipelineStageTypes(p))
	}
	if len(PipelineStageTypes(&types.PipelineRun{})) != 4 {
		t.Fatal("unexpected stage count")
	}
	p.TrainingConfig.Distillation.TrainStudent = true
	if err := NormalizeDistillation(p, project2()); err == nil {
		t.Fatal("training without student accepted")
	}
}

func TestDistill2PreservesMultimodalAndPreferenceRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.jsonl")
	input := `{"instruction":"describe","images":["/data/a.png"],"metadata":{"source":"original"}}
{"messages":[{"role":"user","content":"hello"}],"chosen":"yes","rejected":"no"}`
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadJSONRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["images"] == nil || rows[0]["metadata"] == nil || rows[1]["chosen"] != "yes" {
		t.Fatal(rows)
	}
	if err := os.WriteFile(path, []byte(input+"\n{broken}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadJSONRecords(path); err == nil {
		t.Fatal("malformed input silently accepted")
	}
}

func TestDistill2RejectsInvalidOutputAndMissingTrainingArtifacts(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	p := &types.PipelineRun{UID: 380, ID: "run", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}}}
	root := g.GetRunWorkspace(p.UID, "project", p.ID)
	path := distill2OutputPath(root, "instruct_distill")
	if err := writeJSONLines(path, []map[string]interface{}{{"output": "no messages"}}); err != nil {
		t.Fatal(err)
	}
	e := &StageExecutor{configGen: g}
	if err := e.executeExport2(context.Background(), &types.StageRun{}, project2(), p); err == nil {
		t.Fatal("bad SFT artifact accepted")
	}
	if err := writeJSONLines(path, []map[string]interface{}{{"messages": []interface{}{map[string]interface{}{"role": "assistant", "content": "answer"}}}}); err != nil {
		t.Fatal(err)
	}
	stage := &types.StageRun{}
	if err := e.executeExport2(context.Background(), stage, project2(), p); err != nil {
		t.Fatal(err)
	}
	if stage.OutputManifest["record_count"] != "1" {
		t.Fatal(stage.OutputManifest)
	}
}

type exportDatasetRepo struct {
	mysqlrepo.DatasetRepository
	created *types.Dataset
	writes  int
}

func (r *exportDatasetRepo) GetByID(context.Context, string) (*types.Dataset, error) {
	if r.created == nil {
		return nil, sql.ErrNoRows
	}
	return r.created, nil
}
func (r *exportDatasetRepo) Create(_ context.Context, d *types.Dataset) error {
	r.created = d
	r.writes++
	return nil
}
func TestDistill2RegistersIndependentGeneratedAssetIdempotently(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	p := &types.PipelineRun{UID: 380, ID: "run", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: "instruct_distill"}}}
	p.TrainingConfig.Distillation.TeacherModel = &types.ModelConfig{ModelName: "snapshot-teacher", ServiceID: "owned-service"}
	path := distill2OutputPath(g.GetRunWorkspace(p.UID, "project", p.ID), "instruct_distill")
	if err := writeJSONLines(path, []map[string]interface{}{{"messages": []interface{}{map[string]interface{}{"role": "assistant", "content": "answer"}}, "metadata": map[string]interface{}{"model": "pipeline", "request_id": "original-request"}}}); err != nil {
		t.Fatal(err)
	}
	repo := &exportDatasetRepo{}
	e := &StageExecutor{configGen: g, datasetRepo: repo}
	stage := &types.StageRun{}
	for i := 0; i < 2; i++ {
		if err := e.executeExport2(context.Background(), stage, project2(), p); err != nil {
			t.Fatal(err)
		}
	}
	if repo.writes != 1 || repo.created.SourceType != "generated" || repo.created.UID != 380 || !strings.Contains(repo.created.FilePath, "generated") {
		t.Fatal(repo)
	}
	if _, err := os.Stat(repo.created.FilePath); err != nil {
		t.Fatal(err)
	}
	rows, _, _, err := PreviewRecords(repo.created.FilePath, 1)
	if err != nil {
		t.Fatal(err)
	}
	metadata := rows[0]["metadata"].(map[string]interface{})
	provenance := metadata["gcs_distill"].(map[string]interface{})
	if metadata["model"] != "snapshot-teacher" || provenance["pipeline_id"] != p.ID || provenance["teacher_service_id"] != "owned-service" || metadata["request_id"] != "original-request" {
		t.Fatalf("export lost actual teacher provenance: %+v", metadata)
	}
}

func TestImageReferencesStayInUserWorkspace(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "datasets")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(base, "sample.png")
	if err := os.WriteFile(image, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	rows := []map[string]interface{}{{"images": []interface{}{"sample.png", "https://example.test/image.png"}}}
	if err := normalizeImageReferences(rows, base, root); err != nil {
		t.Fatal(err)
	}
	if rows[0]["images"].([]interface{})[0] != image {
		t.Fatal(rows)
	}
	outside := filepath.Join(t.TempDir(), "other.png")
	if err := os.WriteFile(outside, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := normalizeImageReferences([]map[string]interface{}{{"images": []interface{}{outside}}}, base, root); err == nil {
		t.Fatal("cross-user image accepted")
	}
}

func TestDistill2SnapshotsTeacherAndSelectsStudentPerRun(t *testing.T) {
	project := project2()
	p := &types.PipelineRun{UID: 380, ID: "run"}
	if err := NormalizeDistillation(p, project); err != nil {
		t.Fatal(err)
	}
	project.TeacherModelConfig.Endpoint = "http://changed/v1"
	cfg, err := NewConfigGenerator(t.TempDir()).GenerateDistill2Config(project, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["backend"].(map[string]interface{})["base_url"] != "http://teacher/v1" {
		t.Fatal("teacher snapshot changed")
	}
	w := p.TrainingConfig.Distillation
	w.TrainStudent = true
	w.TrainerImage = "llamafactory"
	w.ModelTemplate = "qwen"
	w.StudentModel = &types.ModelConfig{ProviderType: types.ProviderLocal, ModelName: "selected", ModelPath: "/base/selected"}
	p.ResourceRequest.GPUCount = 1
	p.ResourceRequest.SelectedResources = []types.SelectedResource{{NodeName: "worker", XPUIndices: []int{0}}}
	if err := NormalizeDistillation(p, project); err != nil {
		t.Fatal(err)
	}
	train, _, err := NewConfigGenerator(t.TempDir()).BuildTrainerConfig(project, p)
	if err != nil {
		t.Fatal(err)
	}
	if train["model_name_or_path"] != "/base/selected" {
		t.Fatal(train)
	}
}
