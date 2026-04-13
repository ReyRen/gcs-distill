package apidocs

import (
	"embed"
)

// swaggerFiles contains the embedded Swagger UI entrypoint and OpenAPI spec.
//go:embed swagger/*
var swaggerFiles embed.FS

func MustReadFile(name string) []byte {
	data, err := swaggerFiles.ReadFile("swagger/" + name)
	if err != nil {
		panic(err)
	}

	return data
}