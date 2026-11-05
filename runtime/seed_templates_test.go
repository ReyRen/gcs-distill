package runtime

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

// Exercise the same file reader, field validator and image resolver used by runs.
func TestSeedTemplatesCoverWorkflowsWithUsableRecords(t *testing.T) {
	items, err := SeedTemplates()
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	ids := map[string]bool{}
	root := t.TempDir()
	for _, item := range items {
		if ids[item.ID] || !item.ExamplesOnly || len(item.ExampleRecords) != 1 {
			t.Fatalf("invalid template metadata: %+v", item)
		}
		ids[item.ID] = true
		_, data, err := ReadSeedTemplate(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, item.Filename)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		for _, job := range item.JobTypes {
			if _, ok := FindWorkflow(job); !ok || covered[job] {
				t.Fatalf("unknown or duplicate workflow %s", job)
			}
			covered[job] = true
			count, err := WalkRecords(path, func(row map[string]interface{}) error {
				if err := MapSeedRecord(row, &types.DistillationConfig{JobType: job}); err != nil {
					return err
				}
				return normalizeImageReferences([]map[string]interface{}{row}, root, root)
			})
			if err != nil || count != item.RecordCount || count == 0 {
				t.Fatalf("template %s fails workflow %s: records=%d err=%v", item.ID, job, count, err)
			}
		}
	}
	for _, recipe := range WorkflowRecipes() {
		if !covered[recipe.JobType] {
			t.Fatalf("no seed template for %s", recipe.JobType)
		}
	}
	_, data, err := ReadSeedTemplate("multimodal")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "images.jsonl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = WalkRecords(path, func(row map[string]interface{}) error {
		for _, raw := range row["images"].([]interface{}) {
			data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw.(string), "data:image/png;base64,"))
			if err != nil {
				return err
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return err
			}
			if img.Bounds().Dx() != 300 || img.Bounds().Dy() != 160 {
				t.Fatal("unexpected sample image dimensions")
			}
			original, err := seedTemplateFiles.ReadFile("seed_templates/08_多模态示例图片.png")
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("embedded image differs from bundled PNG", err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSeedTemplateBundleMatchesIndividualDownloads(t *testing.T) {
	data, err := SeedTemplateBundle()
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range z.File {
		if file.Flags&0x800 == 0 {
			t.Fatal("Chinese ZIP filename is not marked as UTF-8", file.Name)
		}
		if strings.Contains(file.Name, "..") || strings.HasPrefix(file.Name, "/") {
			t.Fatal("unsafe ZIP member", file.Name)
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name], err = io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := SeedTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		_, individual, err := ReadSeedTemplate(item.ID)
		if err != nil || !bytes.Equal(individual, files[item.Filename]) {
			t.Fatal("bundle missing or changed template", item.ID, err)
		}
	}
	if len(files["00_使用说明.md"]) == 0 || len(files["08_多模态示例图片.png"]) == 0 || len(files) != len(items)+2 {
		t.Fatal("incomplete or unexpected offline bundle")
	}
	if _, _, err := ReadSeedTemplate("../config.toml"); err == nil {
		t.Fatal("accepted path outside template catalog")
	}
}
