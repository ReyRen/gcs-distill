package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestSingleTurnSFTCanBeReusedAsInstructionSeeds(t *testing.T) {
	for _, job := range []string{"instruct_distill", "advanced_instruct_distill", "augmented_instruct_distill", "balanced_instruct_distill"} {
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(`{"messages":[{"role":"system","content":"original system"},{"role":"user","content":"  如何申请退款？  "},{"role":"assistant","content":"old teacher answer"}],"metadata":{"source":"teacher_model"}}`), &row); err != nil {
			t.Fatal(err)
		}
		original, _ := json.Marshal(row["messages"])
		if err := MapSeedRecord(row, &types.DistillationConfig{JobType: job}); err != nil {
			t.Fatal(job, err)
		}
		if row["instruction"] != "如何申请退款？" || row["response"] != nil || row["output"] != nil || row["metadata"] == nil {
			t.Fatal("old answer reused or source lost", row)
		}
		after, _ := json.Marshal(row["messages"])
		if string(after) != string(original) {
			t.Fatal("source messages changed")
		}
	}
}

func TestMessageSeedConversionDoesNotFlattenAmbiguousOrMultimodalData(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"role":"assistant","content":"answer only"}]`,
		`[{"role":"user","content":" "}]`,
		`[{"role":"user","content":"q1"},{"role":"assistant","content":"a1"},{"role":"user","content":"q2"}]`,
		`[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"data:abc"}}]}]`,
		`[{"role":"user","content":"q"},{"role":"assistant","content":"","tool_calls":[{"id":"call"}]}]`,
		`[{"role":"user","content":"q"},{"role":"tool","content":"result"}]`,
	} {
		var messages interface{}
		if err := json.Unmarshal([]byte(raw), &messages); err != nil {
			t.Fatal(err)
		}
		if err := MapSeedRecord(map[string]interface{}{"messages": messages}, &types.DistillationConfig{JobType: "advanced_instruct_distill"}); err == nil {
			t.Fatal("ambiguous messages accepted", raw)
		}
	}
	var row map[string]interface{}
	_ = json.Unmarshal([]byte(`{"instruction":"chosen input","messages":[{"role":"user","content":"other"}]}`), &row)
	if err := MapSeedRecord(row, &types.DistillationConfig{JobType: "advanced_instruct_distill"}); err != nil || row["instruction"] != "chosen input" {
		t.Fatal("canonical instruction overridden", row, err)
	}
	row["question"] = ""
	if err := MapSeedRecord(row, &types.DistillationConfig{JobType: "advanced_instruct_distill", FieldMapping: map[string]string{"instruction": "question"}}); err == nil {
		t.Fatal("empty explicit mapping silently replaced")
	}
	delete(row, "instruction")
	row["images"] = []interface{}{"/private/image.png"}
	if err := MapSeedRecord(row, &types.DistillationConfig{JobType: "advanced_instruct_distill"}); err == nil {
		t.Fatal("image input silently converted to text-only seed")
	}
}

func TestMessageSeedSnapshotPreservesOriginalFileAndReportsBadRecord(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "original.jsonl")
	snapshot := filepath.Join(dir, "input.jsonl")
	original := []byte("{\"messages\":[{\"role\":\"user\",\"content\":\"question one\"},{\"role\":\"assistant\",\"content\":\"old answer\"}]}\n{\"messages\":[{\"role\":\"user\",\"content\":\"question two\"}]}\n")
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	mapRecord := func(row map[string]interface{}) error {
		return MapSeedRecord(row, &types.DistillationConfig{JobType: "advanced_instruct_distill"})
	}
	count, err := CopyRecords(source, snapshot, mapRecord)
	if err != nil || count != 2 {
		t.Fatal(count, err)
	}
	after, err := os.ReadFile(source)
	if err != nil || string(after) != string(original) {
		t.Fatal("original dataset modified", err)
	}
	if _, err := WalkRecords(snapshot, func(row map[string]interface{}) error {
		if row["instruction"] == nil {
			t.Fatal("snapshot not normalized")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, append(original, []byte("{\"answer\":\"no instruction\"}\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := WalkRecords(source, mapRecord); err == nil || !strings.Contains(err.Error(), "record 3") {
		t.Fatal("missing record index", err)
	}
}
