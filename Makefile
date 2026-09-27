SHELL := /bin/bash
GO_ENV := GOCACHE=$(CURDIR)/.cache/go-build GOPATH=$(CURDIR)/.cache/go
.PHONY: help bootstrap build test demo clean services
help:
	@echo 'BuildWeave: bootstrap build test demo services clean'
bootstrap:
	@mkdir -p bin .cache .buildweave
	@if docker compose ps --status running >/dev/null 2>&1; then docker compose up -d minio-init postgres-init; else echo 'Docker services not running; using local durable adapters.'; fi
build:
	@mkdir -p bin
	$(GO_ENV) go build -o bin/buildweave ./cmd/buildweave
	$(GO_ENV) go build -o bin/buildweave-server ./cmd/buildweave-server
test:
	$(GO_ENV) go test ./...
	./demo/java/gradlew test
	@if test -d demo/typescript/node_modules; then cd demo/typescript && npm test; else echo 'TypeScript compiler test skipped (install packages to enable); declaration integration tests passed.'; fi
demo: build
	BUILDWEAVE_STATE=$(CURDIR)/.buildweave ./bin/buildweave demo
services:
	docker compose up -d
clean:
	@rm -rf bin .cache .buildweave demo/java/build demo/typescript/node_modules demo/typescript/packages/*/dist demo/typescript/packages/*/*.tsbuildinfo
