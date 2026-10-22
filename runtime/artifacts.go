package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Artifact struct {
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
	Bytes        int64  `json:"bytes"`
	Kind         string `json:"kind"`
	Previewable  bool   `json:"previewable"`
}

func artifactKind(path string) (string, bool) {
	name := filepath.Base(path)
	if strings.HasPrefix(path, "data/exported/") || strings.HasPrefix(path, "data/intermediate/") {
		if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl") {
			return "dataset", true
		}
	}
	if strings.HasPrefix(path, "models/checkpoints/") {
		if strings.HasSuffix(name, ".safetensors") || strings.HasPrefix(name, "pytorch_model") && strings.HasSuffix(name, ".bin") || strings.HasPrefix(name, "adapter_model") && strings.HasSuffix(name, ".bin") {
			return "model_weights", false
		}
		for _, n := range []string{"config.json", "adapter_config.json", "tokenizer.json", "tokenizer_config.json", "special_tokens_map.json", "added_tokens.json", "chat_template.jinja", "preprocessor_config.json", "processor_config.json", "video_preprocessor_config.json", "generation_config.json", "vocab.json", "merges.txt", "tokenizer.model", "model.safetensors.index.json", "pytorch_model.bin.index.json"} {
			if name == n {
				return "model_metadata", false
			}
		}
	}
	return "", false
}

func ListArtifacts(root string) ([]Artifact, error) {
	items := []Artifact{}
	for _, sub := range []string{"data/exported", "data/intermediate", "models/checkpoints"} {
		base := filepath.Join(root, filepath.FromSlash(sub))
		if _, err := os.Lstat(base); os.IsNotExist(err) {
			continue
		}
		if _, err := SafeArtifactPath(root, sub); err != nil {
			return nil, err
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			kind, preview := artifactKind(rel)
			if kind == "" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			items = append(items, Artifact{Name: d.Name(), RelativePath: rel, Bytes: info.Size(), Kind: kind, Previewable: preview})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RelativePath < items[j].RelativePath })
	return items, nil
}

func SafeArtifactPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("relative artifact path required")
	}
	path := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("artifact is outside the run")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if resolved != path {
		return "", fmt.Errorf("artifact symlinks are not allowed")
	}
	return path, nil
}
