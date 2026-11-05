package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
)

func sampleSFT(question, answer string) map[string]interface{} {
	return map[string]interface{}{"messages": []interface{}{map[string]interface{}{"role": "user", "content": question}, map[string]interface{}{"role": "assistant", "content": answer}}}
}
func writePreparationRows(t *testing.T, path string, rows []map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
func splitSettings() types.DataSplitConfig {
	return types.DataSplitConfig{Enabled: true, TrainRatio: .8, ValidationRatio: .1, TestRatio: .1, Seed: 42}
}

func TestDataSplitIsReproducibleGroupedAndLossless(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dataset.jsonl")
	rows := []map[string]interface{}{}
	for i := 0; i < 30; i++ {
		row := sampleSFT(fmt.Sprintf("问题 %d", i), "实际回答")
		row["metadata"] = map[string]interface{}{"seed_id": fmt.Sprintf("seed-%d", i/2)}
		rows = append(rows, row)
	}
	// A shared image joins two source groups transitively.
	rows[0]["images"] = []interface{}{"/private/shared.png"}
	rows[2]["images"] = []interface{}{"/private/shared.png"}
	writePreparationRows(t, path, rows)
	preview, err := PreviewDataSplit(path, "dataset", splitSettings())
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "splits")
	m, err := CreateDataSplit(path, base, "dataset", splitSettings())
	if err != nil {
		t.Fatal(err)
	}
	if preview.ID != m.ID || !reflect.DeepEqual(preview.Partitions, m.Partitions) {
		t.Fatal("preview differs from saved split")
	}
	if m.TotalRecords != 30 || m.TotalGroups != 14 {
		t.Fatal(m)
	}
	again, err := CreateDataSplit(path, base, "dataset", splitSettings())
	if err != nil || !again.CreatedAt.Equal(m.CreatedAt) {
		t.Fatal("idempotent save changed snapshot", err)
	}
	assigned := map[string]string{}
	count := 0
	images := map[string]string{}
	for _, name := range splitNames {
		_, err := WalkRecords(filepath.Join(base, m.ID, name+".jsonl"), func(row map[string]interface{}) error {
			seed := row["metadata"].(map[string]interface{})["seed_id"].(string)
			if prev, ok := assigned[seed]; ok && prev != name {
				t.Fatal("source leaked across partitions")
			}
			assigned[seed] = name
			if img, ok := row["images"].([]interface{}); ok {
				key := img[0].(string)
				if prev, ok := images[key]; ok && prev != name {
					t.Fatal("image leaked")
				}
				images[key] = name
			}
			count++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 30 {
		t.Fatal("records lost", count)
	}
	if _, err := ReadDataSplit(base, "../../secret"); err == nil {
		t.Fatal("path traversal accepted")
	}
}

func TestDataSplitRejectsUnsafeAndInsufficientPlans(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dataset.jsonl")
	rows := []map[string]interface{}{sampleSFT("同一个问题", "答案1"), sampleSFT("同一个问题", "答案2"), sampleSFT("同一个问题", "答案3")}
	writePreparationRows(t, path, rows)
	if _, err := CreateDataSplit(path, filepath.Join(dir, "splits"), "id", splitSettings()); err == nil {
		t.Fatal("leaky duplicate prompt split accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "splits")); !os.IsNotExist(err) {
		t.Fatal("failed preview created storage")
	}
	c := splitSettings()
	c.TestRatio = .2
	if err := NormalizeDataSplit(&c); err == nil {
		t.Fatal("invalid sum accepted")
	}
	c = splitSettings()
	c.GroupBy = "metadata.customer_id"
	if _, err := PreviewDataSplit(path, "id", c); err == nil {
		t.Fatal("missing custom grouping accepted")
	}
	if err := os.WriteFile(path, []byte("{broken}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateDataSplit(path, filepath.Join(dir, "splits"), "id", splitSettings()); err == nil {
		t.Fatal("bad input accepted")
	}
}

func TestSplitTrainingUsesTrainAndValidationNeverTest(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	project := project2()
	p := &types.PipelineRun{ID: "run", UID: project.UID, ProjectID: project.ID, DatasetID: "seed", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{Version: "2.0", JobType: "dpo_data_build", TrainStudent: true, StudentModel: &types.ModelConfig{ModelPath: "/private/base"}, FinetuningType: "lora", ModelTemplate: "qwen", DataSplit: ptrSplit(splitSettings())}}}
	root := g.GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	rows := []map[string]interface{}{}
	for i := 0; i < 20; i++ {
		rows = append(rows, map[string]interface{}{"instruction": fmt.Sprintf("Q%d", i), "chosen": "good", "rejected": "bad"})
	}
	path := distill2OutputPath(root, p.TrainingConfig.Distillation.JobType)
	writePreparationRows(t, path, rows)
	m, err := CreateDataSplit(path, filepath.Join(root, "data", "exported", "splits"), "asset", splitSettings())
	if err != nil {
		t.Fatal(err)
	}
	if err := writePrivateJSON(filepath.Join(root, "data", "exported", "manifest.json"), map[string]string{"split_id": m.ID}); err != nil {
		t.Fatal(err)
	}
	cfg, entries, err := g.BuildTrainerConfig(project, p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["eval_dataset"] != "gcs_validation" || cfg["do_eval"] != true || len(entries) != 2 {
		t.Fatal(cfg, entries)
	}
	b, _ := json.Marshal(entries)
	if strings.Contains(string(b), "test.jsonl") || !strings.Contains(string(b), "train.jsonl") || !strings.Contains(string(b), "validation.jsonl") {
		t.Fatal(string(b))
	}
	if err := os.Remove(filepath.Join(root, "data", "exported", "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := g.BuildTrainerConfig(project, p); err == nil {
		t.Fatal("missing split fell back to full dataset")
	}
}
func ptrSplit(c types.DataSplitConfig) *types.DataSplitConfig { return &c }

func TestExpansionUsesNativeOperatorsAndRetainsManagedRoundPaths(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	for _, job := range []string{"instruct_distill", "advanced_instruct_distill", "augmented_instruct_distill", "balanced_instruct_distill"} {
		without := &types.PipelineRun{ID: "off", UID: 380, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: job}}}
		disabled, err := g.GenerateDistill2Config(project2(), without)
		if err != nil {
			t.Fatal(err)
		}
		disabledJSON, _ := json.Marshal(disabled)
		if strings.Contains(string(disabledJSON), "instruction_expansion") {
			t.Fatal("disabled expansion still runs", job)
		}
		c := &types.DataExpansionConfig{Enabled: true, TargetRecords: 500}
		if err := NormalizeDataExpansion(c, job); err != nil {
			t.Fatal(err)
		}
		p := &types.PipelineRun{ID: "run", UID: 380, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: job, DataExpansion: c}}}
		recipe, _ := FindWorkflow(job)
		defaults, err := WorkflowDefaults(recipe)
		if err != nil {
			t.Fatal(err)
		}
		p.TrainingConfig.Distillation.Parameters = defaults
		cfg, err := g.GenerateDistill2Config(project2(), p)
		if err != nil {
			t.Fatal(job, err)
		}
		root := g.GetRunWorkspace(380, "project", "run")
		round, err := expansionRoundConfig(cfg, root, 2, 215)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(round)
		if !strings.Contains(string(b), `"num_output_samples":215`) || !strings.Contains(string(b), "expansion-02") {
			t.Fatal(string(b))
		}
		if job == "advanced_instruct_distill" && !strings.Contains(string(b), "quality_filter") {
			t.Fatal("quality controls removed")
		}
	}
	if err := NormalizeDataExpansion(&types.DataExpansionConfig{Enabled: true, TargetRecords: 500}, "mm_instruct_distill"); err == nil {
		t.Fatal("unsupported expansion accepted")
	}
	if err := NormalizeDataExpansion(&types.DataExpansionConfig{Enabled: true, TargetRecords: 500, MaxCandidates: 100}, "instruct_distill"); err == nil {
		t.Fatal("insufficient budget accepted")
	}
	if got := nextExpansionBatch(types.DataExpansionConfig{TargetRecords: 500, MaxCandidates: 900, BatchSize: 500}, ExpansionProgress{Actual: 350, Candidates: 500, Rounds: []ExpansionRound{{Requested: 500, Added: 350}}}); got != 215 {
		t.Fatal("no adaptive refill", got)
	}
}

func TestExpansionDeduplicatesQuestionsNotAnswersAndHonorsTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "round.jsonl")
	rows := []map[string]interface{}{sampleSFT(" Hello world ", "one"), sampleSFT("hello world", "different answer"), sampleSFT("second", "real response"), sampleSFT("third", ""), sampleSFT("fourth", "valid")}
	writePreparationRows(t, path, rows)
	var buf bytes.Buffer
	seen := map[string]bool{}
	stats, err := collectExpansionRound(path, json.NewEncoder(&buf), seen, 2)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Added != 2 || stats.Usable != 4 || stats.Duplicates != 1 {
		t.Fatal(stats)
	}
	if strings.Contains(buf.String(), "different answer") || strings.Contains(buf.String(), "fourth") {
		t.Fatal("target inflated", buf.String())
	}
	second, err := collectExpansionRound(path, json.NewEncoder(&buf), seen, 10)
	if err != nil || second.Added != 1 || second.Duplicates != 3 {
		t.Fatal("cross-round duplicate counted", second, err)
	}
}

func TestExportAttachesIndependentSplitToDatasetAsset(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	project := project2()
	p := &types.PipelineRun{UID: project.UID, ID: "run", ProjectID: project.ID, DatasetID: "seed", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: "instruct_distill", DataSplit: ptrSplit(splitSettings())}}}
	rows := []map[string]interface{}{}
	for i := 0; i < 20; i++ {
		rows = append(rows, sampleSFT(fmt.Sprintf("Q%d", i), "answer"))
	}
	writePreparationRows(t, distill2OutputPath(g.GetRunWorkspace(p.UID, p.ProjectID, p.ID), "instruct_distill"), rows)
	repo := &exportDatasetRepo{}
	e := &StageExecutor{configGen: g, datasetRepo: repo}
	stage := &types.StageRun{}
	if err := e.executeExport2(context.Background(), stage, project, p); err != nil {
		t.Fatal(err)
	}
	m, err := AttachedDataSplit(filepath.Dir(stage.OutputManifest["asset_path"]))
	if err != nil || m == nil || m.ID != stage.OutputManifest["split_id"] {
		t.Fatal(m, err)
	}
	if err := os.RemoveAll(g.GetRunWorkspace(p.UID, p.ProjectID, p.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(stage.OutputManifest["asset_path"]), "splits", m.ID, "train.jsonl")); err != nil {
		t.Fatal("asset split depends on run", err)
	}
}

func TestTrainerValidationRequiresRealFiniteResults(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadTrainerMetrics(dir, true); err == nil {
		t.Fatal("missing eval result accepted")
	}
	path := filepath.Join(dir, "eval_results.json")
	for _, bad := range []string{`{"eval_loss":null}`, `{"eval_loss":NaN}`, `{"other_metric":1}`} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadTrainerMetrics(dir, true); err == nil {
			t.Fatal("invalid eval loss accepted", bad)
		}
	}
	if err := os.WriteFile(path, []byte(`{"eval_loss":2.085,"eval_runtime":0.1087}`), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadTrainerMetrics(dir, true)
	if err != nil || m["eval_loss"] != 2.085 {
		t.Fatal(m, err)
	}
}

// The worker is mocked only inside this unit test; no fixtures are registered
// as platform datasets or exposed through production teacher APIs.
func TestExpansionBudgetShortfallKeepsPartialArtifactAndFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req gcsclient.ContainerTaskRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if req.ResourceMode != "cpu" || req.XPUNums != 0 {
				t.Error("data expansion allocated GPUs")
			}
			b, err := os.ReadFile(req.Args[1])
			if err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			var cfg map[string]interface{}
			if err := json.Unmarshal(b, &cfg); err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			path := cfg["dataset"].(map[string]interface{})["output_path"].(string)
			writePreparationRows(t, path, []map[string]interface{}{sampleSFT("question one", "answer one"), sampleSFT("question two", "answer two")})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"accepted": true, "container_name": req.ContainerName})
		case http.MethodDelete:
			w.WriteHeader(200)
		case http.MethodGet:
			if strings.Contains(r.URL.Path, "/logs") {
				_, _ = w.Write([]byte(`{"content":"completed test worker"}`))
			} else {
				_, _ = w.Write([]byte(`{"task_states":7}`))
			}
		}
	}))
	defer server.Close()
	project := project2()
	p := &types.PipelineRun{ID: "budget", UID: project.UID, ProjectID: project.ID, TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: "advanced_instruct_distill", DataExpansion: &types.DataExpansionConfig{Enabled: true, TargetRecords: 4, MaxCandidates: 4, MaxRounds: 1, BatchSize: 4}}}}
	e := NewStageExecutor(t.TempDir(), nil, gcsclient.NewClient(server.URL, time.Second), "runtime", nil, nil)
	stage := &types.StageRun{StageType: types.StageDistillData}
	err := e.ExecuteStage(context.Background(), stage, p, project)
	if err == nil || !strings.Contains(err.Error(), "2/4") {
		t.Fatal("shortfall reported as success", err)
	}
	root := e.configGen.GetRunWorkspace(p.UID, p.ProjectID, p.ID)
	count, err := WalkRecords(distill2OutputPath(root, "advanced_instruct_distill"), nil)
	if err != nil || count != 2 {
		t.Fatal("partial artifact lost", count, err)
	}
	progress, ok := stage.Metrics["expansion"].(ExpansionProgress)
	if !ok || progress.Reached || progress.StopReason != "candidate_budget" || progress.Actual != 2 {
		t.Fatal(stage.Metrics)
	}
}
