package service

import (
	"context"
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
