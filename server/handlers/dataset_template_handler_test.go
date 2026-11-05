package handlers

import (
	"encoding/json"
	"mime"
	"net/http/httptest"
	"testing"

	"github.com/ReyRen/gcs-distill/runtime"
	"github.com/gin-gonic/gin"
)

func TestSeedTemplateAPIAndWorkflowCatalogAgree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/dataset-templates", ListSeedTemplates)
	r.GET("/dataset-templates/download", DownloadSeedTemplateBundle)
	r.GET("/dataset-templates/:id/download", DownloadSeedTemplate)
	h := &WorkflowHandler{}
	r.GET("/workflows", h.List)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	var catalog struct {
		Data struct {
			Items []struct {
				JobType string               `json:"job_type"`
				Seed    runtime.SeedTemplate `json:"seed_template"`
			} `json:"items"`
		} `json:"data"`
	}
	w := get("/workflows")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &catalog) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(catalog.Data.Items) != len(runtime.WorkflowRecipes()) {
		t.Fatal("missing workflow entries")
	}
	for _, recipe := range catalog.Data.Items {
		var result struct {
			Data struct {
				Items []runtime.SeedTemplate `json:"items"`
			} `json:"data"`
		}
		w := get("/dataset-templates?job_type=" + recipe.JobType)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Data.Items) != 1 {
			t.Fatal("cannot select template for", recipe.JobType, w.Code, w.Body.String())
		}
		if result.Data.Items[0].ID != recipe.Seed.ID || !recipe.Seed.ExamplesOnly {
			t.Fatal("workflow template mismatch", recipe.JobType)
		}
		w = get("/" + recipe.Seed.DownloadPath)
		if w.Code != 200 || w.Header().Get("Content-Disposition") == "" || w.Header().Get("Content-Type") != "application/x-ndjson; charset=utf-8" {
			t.Fatal("template download failed", recipe.JobType, w.Code)
		}
		_, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if err != nil || params["filename"] != recipe.Seed.Filename {
			t.Fatal("Chinese download filename was lost", params, err)
		}
		_, data, err := runtime.ReadSeedTemplate(recipe.Seed.ID)
		if err != nil || w.Body.String() != string(data) {
			t.Fatal("download differs from validated template", recipe.JobType, err)
		}
	}
	if w := get("/dataset-templates?job_type=unknown"); w.Code != 400 {
		t.Fatal("unknown workflow not rejected", w.Code)
	}
	if w := get("/dataset-templates/unknown/download"); w.Code != 404 {
		t.Fatal("unknown template not rejected", w.Code)
	}
	if w := get("/dataset-templates/download"); w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" {
		t.Fatal("bundle download failed", w.Code)
	} else if _, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition")); err != nil || params["filename"] != "模型蒸馏种子模板.zip" {
		t.Fatal("Chinese bundle filename was lost", params, err)
	}
}
