package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
	"github.com/gin-gonic/gin"
)

func TestEarlySeedValidationNeedsNoTeacherAndChecksOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "dataset.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for i := 0; i < 30; i++ {
		_ = enc.Encode(map[string]interface{}{"instruction": fmt.Sprintf("Question %d", i)})
	}
	f.Close()
	h := &WorkflowHandler{datasets: splitDatasetService{d: &types.Dataset{ID: "seed", UID: 338, FilePath: path}}}
	r := gin.New()
	r.POST("/datasets/:id/validate", h.ValidateDatasetSeed)
	request := func(uid, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		q := httptest.NewRequest("POST", "/datasets/seed/validate?uid="+uid, bytes.NewBufferString(body))
		q.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, q)
		return w
	}
	w := request("338", `{"job_type":"instruct_distill","seed_limit":5}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Data struct {
			Selection struct {
				Source   int `json:"source_records"`
				Selected int `json:"selected_records"`
			} `json:"seed_selection"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Data.Selection.Source != 30 || result.Data.Selection.Selected != 5 {
		t.Fatal(w.Body.String(), err)
	}
	if w := request("505", `{"job_type":"instruct_distill"}`); w.Code != 404 {
		t.Fatal("foreign dataset accepted", w.Code)
	}
	if w := request("338", `{"job_type":"mm_instruct_distill","seed_limit":5}`); w.Code != 422 {
		t.Fatal("text input accepted by multimodal workflow", w.Body.String())
	}
	if w := request("338", `{"job_type":"instruct_distill","seed_limit":-1}`); w.Code != 422 {
		t.Fatal("negative limit accepted")
	}
	if w := request("338", `{"job_type":"unknown"}`); w.Code != 400 {
		t.Fatal("unknown workflow accepted")
	}
}
