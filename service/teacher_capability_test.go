package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTeacherVisionUsesConfigAndUnknownIsNotFalse(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"model_type":"qwen2_vl","vision_config":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"total":2,"items":[{"id":"vision","uid":"42","status":"online","service_type":"local_inference","model_path":%q},{"id":"api","uid":"42","status":"online","service_type":"online_provider"}]}}`, root)
	}))
	defer server.Close()
	items, err := NewTeacherService(server.URL, time.Second).List(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].SupportsVision == nil || !*items[0].SupportsVision || items[0].CapabilitySource != "model_config" || items[1].SupportsVision != nil {
		t.Fatalf("capability was guessed: %+v", items)
	}
}

func TestExternalTeacherUsesOllamaMetadataAndCachesReadOnlyProbe(t *testing.T) {
	probes := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" || r.Method != http.MethodPost {
			t.Errorf("unexpected capability request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
			return
		}
		var req struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		probes++
		switch req.Model {
		case "actual-vision":
			fmt.Fprint(w, `{"capabilities":["completion","vision"]}`)
		case "text-with-vl-name":
			fmt.Fprint(w, `{"capabilities":["completion"]}`)
		case "generic-openai":
			w.WriteHeader(404)
		default:
			t.Error("probed a model not owned by the user", req.Model)
		}
	}))
	defer upstream.Close()
	center := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"items":[
		{"id":"vision","uid":"42","status":"online","service_type":"online_provider","served_model_name":"published","upstream_model_name":"actual-vision","openai_base_url":%q},
		{"id":"text","uid":"42","status":"online","service_type":"online_provider","served_model_name":"text-with-vl-name","openai_base_url":%q},
		{"id":"unknown","uid":"42","status":"online","service_type":"online_provider","served_model_name":"generic-openai","openai_base_url":%q},
		{"id":"foreign","uid":"99","status":"online","service_type":"online_provider","served_model_name":"foreign","openai_base_url":%q}
		]}}`, upstream.URL+"/v1", upstream.URL+"/v1", upstream.URL+"/v1", upstream.URL+"/v1")
	}))
	defer center.Close()
	svc := NewTeacherService(center.URL, time.Second)
	for i := 0; i < 2; i++ {
		items, err := svc.List(context.Background(), 42)
		if err != nil || len(items) != 3 {
			t.Fatal(items, err)
		}
		if items[0].SupportsVision == nil || !*items[0].SupportsVision || items[0].CapabilitySource != "ollama_metadata" {
			t.Fatal("lost actual vision capability", items[0])
		}
		if items[1].SupportsVision == nil || *items[1].SupportsVision {
			t.Fatal("guessed capability from name", items[1])
		}
		if items[2].SupportsVision != nil || items[2].CapabilitySource != "unknown" {
			t.Fatal("failed metadata probe became a known capability", items[2])
		}
	}
	if probes != 3 {
		t.Fatal("metadata probes were not cached", probes)
	}
}
