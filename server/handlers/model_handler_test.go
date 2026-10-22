package handlers

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/service"
	"github.com/gin-gonic/gin"
)

func TestStudentCatalogRequiresUIDAndSeparatesUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storage := config.StorageConfig{RootPath: t.TempDir()}
	base, _ := storage.UserStudentModels(380)
	path := filepath.Join(base, "private-model")
	if err := os.MkdirAll(path, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "config.json"), []byte(`{"model_type":"qwen2"}`), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewModelHandler(service.NewModelService(&storage))
	r := gin.New()
	r.GET("/models/student", h.ListStudentModels)
	r.GET("/models/student/:id", h.GetStudentModel)
	for _, test := range []struct {
		url  string
		code int
	}{
		{"/models/student", 400},
		{"/models/student?uid=0", 400},
		{"/models/student?uid=380", 200},
		{"/models/student?uid=381", 200},
		{"/models/student/private-model?uid=381", 404},
		{"/models/student/private-model?uid=380", 200},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", test.url, nil))
		if w.Code != test.code || strings.Contains(test.url, "uid=381") && strings.Contains(w.Body.String(), "private-model") {
			t.Fatalf("%s: %d %s", test.url, w.Code, w.Body.String())
		}
	}
}
