package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestManagerWritesEasyDistillJSONArray(t *testing.T) {
	root := t.TempDir()
	manager := NewManifestManager(root)

	err := manager.CreateSeedManifest(380, "project-1", "run-1", []Instruction{
		{Instruction: "Explain distillation."},
		{Instruction: "Write a Go function."},
	})
	if err != nil {
		t.Fatalf("CreateSeedManifest returned error: %v", err)
	}

	path := filepath.Join(root, "user-380", "train-center", "model-distill", "projects", "project-1", "runs", "run-1", "data", "seed", "instructions.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed file: %v", err)
	}

	var got []Instruction
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("seed file is not a JSON array: %v\n%s", err, string(data))
	}
	if len(got) != 2 {
		t.Fatalf("record count = %d", len(got))
	}
}

func TestManifestManagerReadsJSONArrayAndJSONL(t *testing.T) {
	root := t.TempDir()
	manager := NewManifestManager(root)
	base := filepath.Join(root, "user-380", "train-center", "model-distill", "projects", "project-1", "runs", "run-1", "data", "generated")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	arrayPath := filepath.Join(base, "labeled.json")
	if err := os.WriteFile(arrayPath, []byte(`[{"instruction":"a","output":"answer"}]`), 0644); err != nil {
		t.Fatalf("write array: %v", err)
	}
	items, err := manager.LoadLabeledData(380, "project-1", "run-1")
	if err != nil {
		t.Fatalf("LoadLabeledData array returned error: %v", err)
	}
	if len(items) != 1 || items[0].Output != "answer" {
		t.Fatalf("array items = %#v", items)
	}

	if err := os.WriteFile(arrayPath, []byte("{\"instruction\":\"b\",\"output\":\"jsonl answer\"}\n"), 0644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	items, err = manager.LoadLabeledData(380, "project-1", "run-1")
	if err != nil {
		t.Fatalf("LoadLabeledData jsonl returned error: %v", err)
	}
	if len(items) != 1 || items[0].Output != "jsonl answer" {
		t.Fatalf("jsonl items = %#v", items)
	}
}
