package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func writeJSONLines(path string, rows []map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return f.Close()
}

func TestInvalidRecordNeverReplacesPublishedDataset(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "source.jsonl")
	output := filepath.Join(root, "published.jsonl")
	os.WriteFile(output, []byte("original"), 0600)
	os.WriteFile(input, []byte("{\"instruction\":\"valid\"}\ninvalid\n"), 0600)
	if _, err := CopyRecords(input, output, nil); err == nil {
		t.Fatal("malformed input accepted")
	}
	got, _ := os.ReadFile(output)
	if string(got) != "original" {
		t.Fatal("published asset was replaced")
	}
	left, _ := filepath.Glob(filepath.Join(root, ".records-*"))
	if len(left) > 0 {
		t.Fatal("partial export left behind")
	}
}

func TestPreviewScansAllRecordsWithoutTruncatingTotalOrFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	os.WriteFile(path, []byte(`[{"instruction":"first"},{"instruction":"second","images":["a.png"]}]`), 0600)
	rows, total, fields, err := PreviewRecords(path, 1)
	if err != nil || len(rows) != 1 || total != 2 || len(fields) != 2 {
		t.Fatal(rows, total, fields, err)
	}
	os.WriteFile(path, []byte(`[{"a":1}] {"extra":1}`), 0600)
	if _, err := WalkRecords(path, nil); err == nil {
		t.Fatal("accepted trailing data")
	}
}

func TestInputMappingPreservesMetadataAndRejectsMissingColumns(t *testing.T) {
	row := map[string]interface{}{"question": "hello", "metadata": map[string]interface{}{"origin": "seed"}}
	w := &types.DistillationConfig{JobType: "instruct_distill", FieldMapping: map[string]string{"instruction": "question"}}
	if err := MapSeedRecord(row, w); err != nil || row["metadata"] == nil || row["question"] != "hello" {
		t.Fatal(row, err)
	}
	if err := MapSeedRecord(map[string]interface{}{"text": "wrong column"}, w); err == nil {
		t.Fatal("missing mapped column accepted")
	}
}

func TestArtifactsExcludeSecretsAndIncludeDownloadableWeights(t *testing.T) {
	root := t.TempDir()
	for path, data := range map[string]string{"configs/teacher.json": "secret", "models/checkpoints/adapter_model.safetensors": "weights", "models/checkpoints/adapter_config.json": "{}", "models/checkpoints/private.env": "secret", "data/exported/dataset.jsonl": "{}"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		os.MkdirAll(filepath.Dir(full), 0755)
		os.WriteFile(full, []byte(data), 0600)
	}
	files, err := ListArtifacts(root)
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	if _, err := SafeArtifactPath(root, "../config.toml"); err == nil {
		t.Fatal("traversal accepted")
	}
}

// ReadJSONRecords preserves messages, images, tool trajectories, scores and provenance.
// Malformed records fail the job instead of silently dropping training examples.
func ReadJSONRecords(path string) ([]map[string]interface{}, error) {
	rows := []map[string]interface{}{}
	_, err := WalkRecords(path, func(row map[string]interface{}) error { rows = append(rows, row); return nil })
	return rows, err
}
