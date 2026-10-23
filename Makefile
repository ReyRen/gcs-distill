GO ?= go
VERSION ?= v2.0.4-x86-offline
.PHONY: build test swagger fmt
build: swagger
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/gcs-distill-server ./cmd/server
test:
	$(GO) test ./...
swagger:
	$(GO) run ./cmd/openapi
fmt:
	gofmt -w cmd internal repository runtime server service
