package runtime

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ReyRen/gcs-distill/internal/types"
)

func embeddedPNG(t *testing.T, value color.Color) string {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, value)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func TestMultimodalExportPreservesImageOrderAndIndependentAssets(t *testing.T) {
	root := t.TempDir()
	assetDir := filepath.Join(root, "datasets", "generated", "asset", "images")
	first := embeddedPNG(t, color.RGBA{R: 255, A: 255})
	second := embeddedPNG(t, color.RGBA{B: 255, A: 255})
	row := map[string]interface{}{"metadata": map[string]interface{}{"source": "teacher"}, "messages": []interface{}{
		map[string]interface{}{"role": "user", "content": []interface{}{
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": first}},
			map[string]interface{}{"type": "text", "text": "Compare these images."},
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": second}},
		}}, map[string]interface{}{"role": "assistant", "content": "A red image and a blue image."},
	}}
	if err := normalizeMultimodalRecord(row, assetDir, root); err != nil {
		t.Fatal(err)
	}
	images := row["images"].([]interface{})
	if len(images) != 2 || images[0] == images[1] || row["metadata"] == nil {
		t.Fatal("images or metadata lost")
	}
	for index, reference := range []string{first, second} {
		expected, _ := base64.StdEncoding.DecodeString(strings.SplitN(reference, ",", 2)[1])
		actual, err := os.ReadFile(images[index].(string))
		if err != nil || !bytes.Equal(actual, expected) || filepath.Dir(images[index].(string)) != assetDir {
			t.Fatal("attachment order/content/path changed", err)
		}
	}
	content := row["messages"].([]interface{})[0].(map[string]interface{})["content"]
	if content != "<image>Compare these images.<image>" {
		t.Fatal("image/text order changed", content)
	}
	if err := normalizeMultimodalRecord(row, assetDir, root); err != nil {
		t.Fatal("normalization is not repeatable", err)
	}
}

func TestMultimodalExportRejectsForeignImagesAndMissingMarkers(t *testing.T) {
	root := t.TempDir()
	foreign := filepath.Join(t.TempDir(), "foreign.png")
	if err := os.WriteFile(foreign, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := persistMultimodalImage(foreign, filepath.Join(root, "images"), root); err == nil {
		t.Fatal("cross-user image accepted")
	}
	row := map[string]interface{}{"images": []interface{}{embeddedPNG(t, color.White)}, "messages": []interface{}{
		map[string]interface{}{"role": "user", "content": "No matching image marker"},
	}}
	if err := normalizeMultimodalRecord(row, filepath.Join(root, "images"), root); err == nil {
		t.Fatal("image/marker mismatch accepted")
	}
}

func TestMultimodalInputRejectsExternalImagesBeforeCreatingTask(t *testing.T) {
	for _, reference := range []interface{}{"https://example.invalid/image.png", 42, ""} {
		row := map[string]interface{}{"instruction": "Describe it", "images": []interface{}{reference}}
		if err := MapSeedRecord(row, &types.DistillationConfig{JobType: "mm_instruct_distill"}); err == nil {
			t.Fatal("unsupported image reference accepted", reference)
		}
	}
}
