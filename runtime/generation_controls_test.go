package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestSeedLimitSelectsFiveWithoutChangingSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "seed.jsonl")
	output := filepath.Join(dir, "input.jsonl")
	rows := []map[string]interface{}{}
	for i := 0; i < 30; i++ {
		rows = append(rows, map[string]interface{}{"instruction": "real input"})
	}
	writePreparationRows(t, source, rows)
	before, _ := os.ReadFile(source)
	w := &types.DistillationConfig{JobType: "instruct_distill", SeedLimit: 5}
	s, err := InspectSeedSelection(context.Background(), source, w)
	if err != nil || s.SourceRecords != 30 || s.SelectedRecords != 5 {
		t.Fatal(s, err)
	}
	used, total, err := CopySeedRecords(source, output, 5, func(row map[string]interface{}) error { return MapSeedRecord(row, w) })
	if err != nil || used != 5 || total != 30 {
		t.Fatal(used, total, err)
	}
	if count, err := WalkRecords(output, nil); err != nil || count != 5 {
		t.Fatal(count, err)
	}
	after, _ := os.ReadFile(source)
	if string(before) != string(after) {
		t.Fatal("source changed")
	}
	w.SeedLimit = 50
	s, err = InspectSeedSelection(context.Background(), source, w)
	if err != nil || s.SelectedRecords != 30 {
		t.Fatal(s, err)
	}
	if err := os.WriteFile(source, append(before, []byte("{broken}")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CopySeedRecords(source, output, 5, nil); err == nil {
		t.Fatal("bad JSON after limit ignored")
	}
	if count, err := WalkRecords(output, nil); err != nil || count != 5 {
		t.Fatal("failed copy replaced output", count, err)
	}
}

func TestTeacherDefaultsAndExplicitRetryControls(t *testing.T) {
	p := &types.PipelineRun{}
	if err := NormalizeDistillation(p, project2()); err != nil {
		t.Fatal(err)
	}
	w := p.TrainingConfig.Distillation
	if w.TeacherModel.Concurrency != 1 || w.TeacherModel.MaxTokens != 2048 || *w.TeacherModel.RetryAttempts != 1 {
		t.Fatal(w.TeacherModel)
	}
	zero := 0
	w.TeacherModel.RetryAttempts = &zero
	w.TeacherModel.MaxTokens = 512
	for _, job := range []string{"instruct_distill", "advanced_instruct_distill", "agent_distill", "mm_instruct_distill"} {
		w.JobType = job
		cfg, err := NewConfigGenerator(t.TempDir()).GenerateDistill2Config(project2(), p)
		if err != nil {
			t.Fatal(err)
		}
		if cfg["backend"].(map[string]interface{})["max_retries"] != 0 {
			t.Fatal("SDK retries still enabled")
		}
		if generation, ok := cfg["generation"].(map[string]interface{}); ok && generation["retry_attempts"] != 0 {
			t.Fatal("retry zero ignored", job)
		}
	}
	w.SeedLimit = -1
	if err := NormalizeDistillation(p, project2()); err == nil {
		t.Fatal("negative seed limit accepted")
	}
}

func TestGenerationLogProgressIsIncrementalAndCircuitPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	start := "Running pipeline stage 1/2: generate\nHTTP Request: GET http://teacher/v1/models \"HTTP/1.1 200 OK\"\nHTTP Request: POST http://teacher/v1/chat/completions \"HTTP/1.1 200 OK\"\n"
	if err := os.WriteFile(path, []byte(start+"Generation failed for instruction 1 (attempt 1/2): Request timed"), 0600); err != nil {
		t.Fatal(err)
	}
	o := &generationObserver{}
	changed, err := o.read(path)
	if err != nil || !changed || o.progress.SuccessfulRequests != 1 || o.progress.RetryAttempts != 0 {
		t.Fatal(o.progress, err)
	}
	if changed, err := o.read(path); err != nil || changed {
		t.Fatal("partial line double counted")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(" out.. Retrying in 1.0s.\nFailed to generate for instruction 1 after 2 attempts: Request timed out.\nGeneration failed for instruction 2 (attempt 1/2): Request timed out.. Retrying in 1s.\nHTTP Request: POST http://teacher/v1/chat/completions \"HTTP/1.1 200 OK\"\n")
	f.Close()
	stage := &types.StageRun{StageType: types.StageDistillData, ContainerID: "owned-container", LogPath: dir}
	e := &StageExecutor{}
	err = e.observeGeneration(context.Background(), stage, o)
	if err == nil || !strings.Contains(err.Error(), "已停止本任务") {
		t.Fatal("circuit did not open", err)
	}
	p := stage.Metrics["generation"].(GenerationProgress)
	if p.SuccessfulRequests != 2 || p.FailedRequests != 1 || p.RetryAttempts != 2 || p.TimeoutAttempts != 3 || !p.CircuitOpen || p.QueueDepth != nil {
		t.Fatal(p)
	}
	o.consume("Skipping invalid result for instruction: 0")
	o.consume("Built 2 SFT samples from 5 generation results.")
	if o.progress.InvalidResults != 1 || o.progress.BuiltSamples == nil || *o.progress.BuiltSamples != 2 {
		t.Fatal("HTTP success mistaken for final samples", o.progress)
	}
}

func TestTimeoutCircuitStopsOnlyOwnedDataContainer(t *testing.T) {
	stopped := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/owned" {
			t.Errorf("unexpected container operation %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if r.Method == "DELETE" {
			stopped = true
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`{"task_states":3}`))
	}))
	defer server.Close()
	dir := t.TempDir()
	line := "Generation failed for instruction 1 (attempt 1/2): Request timed out.. Retrying in 1s.\n"
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte(strings.Repeat(line, 3)), 0600); err != nil {
		t.Fatal(err)
	}
	e := NewStageExecutor(t.TempDir(), nil, gcsclient.NewClient(server.URL, time.Second), "runtime", nil, nil)
	stage := &types.StageRun{StageType: types.StageDistillData, ContainerID: "owned", LogPath: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := e.waitForContainerTask(ctx, "owned", "pipeline", stage); err == nil || !stopped {
		t.Fatal("timeout did not stop only its own task", err, stopped)
	}
}

func TestCompatibilityChecksAllRowsWithoutChangingInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dataset.jsonl")
	rows := []map[string]interface{}{}
	for i := 0; i < 100; i++ {
		rows = append(rows, map[string]interface{}{"instruction": "question"})
	}
	rows = append(rows, map[string]interface{}{"persona": "agent only"})
	writePreparationRows(t, path, rows)
	all, err := InspectWorkflowCompatibility(context.Background(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range all.Items {
		if item.JobType == "instruct_distill" && (item.Compatible || item.InvalidRecord != 101) {
			t.Fatal("only preview rows checked", item)
		}
	}
	first, err := InspectWorkflowCompatibility(context.Background(), path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if first.SeedSelection.SourceRecords != 101 || first.SeedSelection.SelectedRecords != 5 {
		t.Fatal(first.SeedSelection)
	}
	for _, item := range first.Items {
		if item.JobType == "instruct_distill" && !item.Compatible {
			t.Fatal(item)
		}
		if item.JobType == "mm_instruct_distill" && item.Compatible {
			t.Fatal("missing images accepted")
		}
	}
	writePreparationRows(t, path, []map[string]interface{}{{"instruction": "describe", "images": []interface{}{"data:image/png;base64,example"}}})
	image, err := InspectWorkflowCompatibility(context.Background(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range image.Items {
		if item.JobType == "instruct_distill" && item.Compatible {
			t.Fatal("images silently discarded")
		}
		if item.JobType == "mm_instruct_distill" && !item.Compatible {
			t.Fatal(item)
		}
	}
}
