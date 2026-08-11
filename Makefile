.DEFAULT_GOAL := help

## run: run the service locally
run:
	go run ./cmd/scouter

## build: compile to ./bin
build:
	go build -o bin/scouter ./cmd/scouter

## test: run all tests
test:
	go test ./... -race -count=1

## fmt: format and tidy
fmt:
	go fmt ./...
	go mod tidy

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

.PHONY: run build test fmt help