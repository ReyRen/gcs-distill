package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const specPath = "server/apidocs/swagger/openapi.json"

func main() {
	data, err := os.ReadFile(specPath)
	if err != nil {
		exitf("read OpenAPI spec: %v", err)
	}
	checkForbiddenTerms(data)

	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		exitf("parse OpenAPI spec: %v", err)
	}
	if spec["openapi"] == "" {
		exitf("OpenAPI spec missing openapi version")
	}
	if _, ok := spec["paths"].(map[string]any); !ok {
		exitf("OpenAPI spec missing paths object")
	}
	if _, ok := spec["components"].(map[string]any); !ok {
		exitf("OpenAPI spec missing components object")
	}

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spec); err != nil {
		exitf("format OpenAPI spec: %v", err)
	}
	if err := os.WriteFile(specPath, out.Bytes(), 0644); err != nil {
		exitf("write OpenAPI spec: %v", err)
	}
}

func checkForbiddenTerms(data []byte) {
	forbidden := []string{
		"cmd/worker",
		"proto/worker",
		"internal/docker",
		"Dockerfile.worker",
		"WorkerNode",
		"gcs-distill-worker",
		"redis:",
		"grpc:",
		"50051",
	}
	for _, term := range forbidden {
		if bytes.Contains(data, []byte(term)) {
			exitf("OpenAPI spec contains removed distill runtime term %q", term)
		}
	}
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
