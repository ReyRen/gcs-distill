package runtime

import (
	"reflect"
	"testing"
)

func TestParseInstructionFileSupportsJSONArrayAndJSONL(t *testing.T) {
	jsonArray := []byte(`[{"instruction":"first"},{"instruction":""},{"instruction":"second"}]`)
	items, scanned, err := parseInstructionFile(jsonArray)
	if err != nil {
		t.Fatalf("parse JSON array returned error: %v", err)
	}
	if scanned != 3 || len(items) != 2 {
		t.Fatalf("array scanned=%d len=%d items=%#v", scanned, len(items), items)
	}

	jsonl := []byte("{\"instruction\":\"first\"}\n{\"instruction\":\"second\"}\n")
	items, scanned, err = parseInstructionFile(jsonl)
	if err != nil {
		t.Fatalf("parse JSONL returned error: %v", err)
	}
	if scanned != 2 || len(items) != 2 {
		t.Fatalf("jsonl scanned=%d len=%d items=%#v", scanned, len(items), items)
	}
}

func TestEasyDistillTrainArgsUseModuleLaunch(t *testing.T) {
	got := easyDistillTrainArgs("/workspace/configs/student_train.json", 2)
	want := []string{
		"launch",
		"--multi_gpu",
		"--num_processes", "2",
		"--module", "easydistill.kd.train",
		"--config", "/workspace/configs/student_train.json",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("easyDistillTrainArgs = %#v, want %#v", got, want)
	}
}
