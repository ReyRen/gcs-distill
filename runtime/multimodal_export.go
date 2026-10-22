package runtime

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Native EasyDistill emits OpenAI content arrays. LLaMA-Factory consumes
// string messages with <image> markers and a separate ordered images column.
// Store embedded images with the independent generated dataset, not the run.
func normalizeMultimodalRecord(row map[string]interface{}, imageDir, userRoot string) error {
	messages, ok := row["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return fmt.Errorf("multimodal output has no messages")
	}
	images := []interface{}{}
	markers := 0
	for _, raw := range messages {
		message, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid multimodal message")
		}
		if parts, ok := message["content"].([]interface{}); ok {
			var text strings.Builder
			for _, rawPart := range parts {
				part, ok := rawPart.(map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid multimodal content part")
				}
				switch part["type"] {
				case "text":
					value, ok := part["text"].(string)
					if !ok {
						return fmt.Errorf("invalid multimodal text")
					}
					text.WriteString(value)
				case "image_url":
					if message["role"] != "user" {
						return fmt.Errorf("training input images must be in user messages")
					}
					image, ok := part["image_url"].(map[string]interface{})
					if !ok {
						return fmt.Errorf("invalid image_url content")
					}
					reference, _ := image["url"].(string)
					path, err := persistMultimodalImage(reference, imageDir, userRoot)
					if err != nil {
						return err
					}
					images = append(images, path)
					text.WriteString("<image>")
				default:
					return fmt.Errorf("unsupported multimodal content type: %v", part["type"])
				}
			}
			message["content"] = text.String()
		}
		content, ok := message["content"].(string)
		if !ok {
			return fmt.Errorf("multimodal message content must be text or image/text parts")
		}
		markers += strings.Count(content, "<image>")
	}
	if len(images) == 0 {
		// Already normalized exports can be revalidated without losing attachments.
		references, _ := row["images"].([]interface{})
		for _, raw := range references {
			reference, ok := raw.(string)
			if !ok {
				return fmt.Errorf("invalid training image reference")
			}
			path, err := persistMultimodalImage(reference, imageDir, userRoot)
			if err != nil {
				return err
			}
			images = append(images, path)
		}
	}
	if len(images) == 0 || markers != len(images) {
		return fmt.Errorf("image markers (%d) and attachments (%d) must match", markers, len(images))
	}
	row["images"] = images
	return nil
}

func persistMultimodalImage(reference, imageDir, userRoot string) (string, error) {
	const maxImageBytes = 32 << 20
	var data []byte
	var err error
	if strings.HasPrefix(reference, "data:image/") {
		header, encoded, ok := strings.Cut(reference, ",")
		if !ok || !strings.HasSuffix(header, ";base64") || len(encoded) > base64.StdEncoding.EncodedLen(maxImageBytes) {
			return "", fmt.Errorf("invalid or oversized embedded image")
		}
		data, err = base64.StdEncoding.DecodeString(encoded)
	} else {
		if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
			return "", fmt.Errorf("离线数据导出需要本地图片或内嵌图片，请先将图片上传到当前用户共享目录")
		}
		path := strings.TrimPrefix(reference, "file://")
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		relative, relErr := filepath.Rel(userRoot, path)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return "", fmt.Errorf("image must belong to the current user workspace")
		}
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > maxImageBytes {
			return "", fmt.Errorf("invalid or oversized image file")
		}
		data, err = os.ReadFile(path)
	}
	if err != nil || len(data) == 0 || len(data) > maxImageBytes {
		return "", fmt.Errorf("invalid image bytes: %v", err)
	}
	extensions := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp"}
	extension, ok := extensions[http.DetectContentType(data)]
	if !ok {
		return "", fmt.Errorf("unsupported image format")
	}
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(imageDir, fmt.Sprintf("%x%s", sha256.Sum256(data), extension))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}
