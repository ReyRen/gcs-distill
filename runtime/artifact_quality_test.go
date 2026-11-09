package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func artifactFixture(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestArtifactCountsDistinguishEmptyInvalidAndMetadata(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"dataset.jsonl": "{\"a\":1}\n{\"a\":2}\n", "empty.jsonl": " \n", "array.json": "[]",
		"bad.jsonl": "{\"a\":1}\nbad", "manifest.json": "{\"record_count\":999}", "expansion.json": "{}",
	} {
		artifactFixture(t, root, "data/exported/"+name, data)
	}
	files, err := ListArtifacts(root)
	if err != nil {
		t.Fatal(err)
	}
	CountArtifactRecords(root, files)
	for _, file := range files {
		switch file.Name {
		case "dataset.jsonl":
			if file.RecordCount == nil || *file.RecordCount != 2 || file.CountStatus != "counted" {
				t.Fatal(file)
			}
		case "empty.jsonl", "array.json":
			if file.RecordCount == nil || *file.RecordCount != 0 || file.CountStatus != "counted" {
				t.Fatal(file)
			}
		case "bad.jsonl":
			if file.RecordCount != nil || file.CountStatus != "unavailable" {
				t.Fatal("partial count reported as complete", file)
			}
		default:
			if file.RecordCount != nil || file.Kind != "dataset_metadata" || file.CountStatus != "not_applicable" {
				t.Fatal(file)
			}
		}
	}
	for _, name := range []string{"empty.jsonl", "array.json"} {
		rows, total, _, quality, err := PreviewArtifact(root, "data/exported/"+name, 10)
		if err != nil || total != 0 || len(rows) != 0 || len(quality.Items) != 0 {
			t.Fatal(rows, total, quality, err)
		}
		if _, err := WalkRecords(filepath.Join(root, "data/exported", name), nil); err == nil {
			t.Fatal("empty seed accepted")
		}
	}
}

func TestPreviewRestoresExactScoresWithoutChangingExistingData(t *testing.T) {
	root := t.TempDir()
	source := "data/intermediate/expansion-01/04_instruct_eval.jsonl"
	artifactFixture(t, root, source, `{"id":"1","instruction":"Q","response":"A","helpfulness":0,"correctness":false}`)
	artifactFixture(t, root, "data/intermediate/expansion-02/04_instruct_eval.jsonl", `{"id":"1","instruction":"Q","response":"different","helpfulness":9}`)
	path := artifactFixture(t, root, "data/exported/dataset.jsonl", `{"messages":[{"role":"system","content":"S"},{"role":"user","content":"Q"},{"role":"assistant","content":"A"}]}`+"\n"+`{"messages":[{"role":"user","content":"unknown"},{"role":"assistant","content":"A"}]}`)
	before, _ := os.ReadFile(path)
	rows, total, _, quality, err := PreviewArtifact(root, "data/exported/dataset.jsonl", 1)
	if err != nil || len(rows) != 1 || total != 2 || quality.Scored != 1 || quality.Scope != "preview_records" {
		t.Fatal(quality, total, err)
	}
	q := quality.Items[0]
	if q.Metrics["helpfulness"] != float64(0) || q.Metrics["correctness"] != false || q.Sources[0] != source {
		t.Fatal("zero/false scores lost or wrong answer matched", q)
	}
	report, err := QualityReport(root)
	if err != nil || report.ExportedDataset.Records != 2 || report.ExportedDataset.Scored != 1 || report.ExportedDataset.Unscored != 1 {
		t.Fatal(report, err)
	}
	if report.ExportedDataset.Metrics["helpfulness"].Count != 1 {
		t.Fatal("missing score counted as zero", report)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read-only scoring modified immutable dataset")
	}
	// Conflicting actual scores must not be silently assigned by round or ID.
	artifactFixture(t, root, "data/intermediate/expansion-03/04_instruct_eval.jsonl", `{"id":"1","instruction":"Q","response":"A","helpfulness":9,"correctness":true}`)
	_, _, _, quality, err = PreviewArtifact(root, "data/exported/dataset.jsonl", 10)
	if err != nil || quality.Ambiguous != 1 || quality.Items[0].Metrics != nil {
		t.Fatal(quality, err)
	}
	// A corrupted file must not leak a score decoded before its bad last line.
	artifactFixture(t, root, "data/intermediate/expansion-03/04_instruct_eval.jsonl", `{"instruction":"Q","response":"A","helpfulness":9}`+"\nbroken")
	_, _, _, quality, err = PreviewArtifact(root, "data/exported/dataset.jsonl", 10)
	if err != nil || quality.Scored != 1 || quality.Ambiguous != 0 {
		t.Fatal(quality, err)
	}
}

func TestExportPersistsScoresInAssetAndImmutableSplits(t *testing.T) {
	g := NewConfigGenerator(t.TempDir())
	p := &types.PipelineRun{ID: "run", UID: 380, DatasetID: "seed", TrainingConfig: types.TrainingConfig{Distillation: &types.DistillationConfig{JobType: "advanced_instruct_distill", DataSplit: ptrSplit(splitSettings())}}}
	root := g.GetRunWorkspace(p.UID, "project", p.ID)
	rows, scored := []map[string]interface{}{}, []map[string]interface{}{}
	for i, pair := range [][2]string{{"Q1", "A1"}, {"Q2", "A2"}, {"Q3", "A3"}, {"Q4", "A4"}, {"Q5", "A5"}} {
		rows = append(rows, map[string]interface{}{"messages": []interface{}{map[string]interface{}{"role": "user", "content": pair[0]}, map[string]interface{}{"role": "assistant", "content": pair[1]}}})
		scored = append(scored, map[string]interface{}{"instruction": pair[0], "response": pair[1], "helpfulness": i + 5, "correctness": true})
	}
	if err := writeJSONLines(distill2OutputPath(root, ""), rows); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONLines(filepath.Join(root, "data/intermediate/04_instruct_eval.jsonl"), scored); err != nil {
		t.Fatal(err)
	}
	repo := &exportDatasetRepo{}
	e := &StageExecutor{configGen: g, datasetRepo: repo}
	stage := &types.StageRun{}
	if err := e.executeExport2(context.Background(), stage, project2(), p); err != nil {
		t.Fatal(err)
	}
	check := func(path string) {
		t.Helper()
		_, err := WalkRecords(path, func(row map[string]interface{}) error {
			if fieldValue(row, "metadata.quality.status") != "scored" || len(recordScores(row)) != 2 {
				t.Fatalf("score missing in %s: %+v", path, row)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	check(repo.created.FilePath)
	m, err := AttachedDataSplit(filepath.Join(root, "data/exported"))
	if err != nil || m == nil {
		t.Fatal(m, err)
	}
	for name, partition := range m.Partitions {
		if partition.Records > 0 {
			check(filepath.Join(root, "data/exported/splits", m.ID, name+".jsonl"))
		}
	}
}
