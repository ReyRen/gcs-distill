package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func TestTeacherOwnershipBeforeCredentialResolution(t *testing.T) {
	connections := 0
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/owned":
			fmt.Fprint(w, `{"data":{"id":"owned","uid":"369","status":"online"}}`)
		case "/services/other":
			fmt.Fprint(w, `{"data":{"id":"other","uid":"505","status":"online"}}`)
		case "/services/owned/connection":
			connections++
			fmt.Fprintf(w, `{"data":{"status":"online","openai_base_url":%q,"served_model_name":"teacher","api_key":"private"}}`, upstream.URL+"/v1")
		case "/v1/models":
			if r.Header.Get("Authorization") != "Bearer private" {
				t.Error("missing teacher authentication")
			}
			fmt.Fprint(w, `{"data":[{"id":"teacher"}]}`)
		case "/v1/chat/completions":
			if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer private" {
				t.Error("invalid inference probe")
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	svc := NewTeacherService(upstream.URL, time.Second)
	foreign := &types.ModelConfig{ServiceID: "other"}
	if err := svc.Resolve(context.Background(), 369, foreign); err == nil || connections != 0 {
		t.Fatal("foreign credentials were resolved", err, connections)
	}
	own := &types.ModelConfig{ServiceID: "owned"}
	if err := svc.Test(context.Background(), 369, own); err != nil {
		t.Fatal(err)
	}
	if own.APISecretRef != "private" || !strings.HasSuffix(own.Endpoint, "/v1") {
		t.Fatal("snapshot was not resolved")
	}
}

func TestTeacherInferenceTimeoutIsSeparateFromCatalogTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"existing-model"}]}`)
		case "/v1/chat/completions":
			time.Sleep(100 * time.Millisecond)
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	svc := NewTeacherService("", 50*time.Millisecond)
	for _, timeout := range []int{0, 1} {
		model := &types.ModelConfig{Endpoint: upstream.URL + "/v1", ModelName: "existing-model", TimeoutSeconds: timeout}
		if err := svc.Test(context.Background(), 369, model); err != nil {
			t.Fatalf("model loading was limited by catalog timeout (teacher timeout %d): %v", timeout, err)
		}
	}
	if svc.client.Timeout != 50*time.Millisecond {
		t.Fatal("teacher test changed the shared catalog client timeout")
	}
}
