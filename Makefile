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

.PHONY: test run fmt docker-build docker-run

IMAGE   ?= scouter
VERSION ?= $(shell git rev-parse --short HEAD)

## docker-build: build the container image tagged with the git SHA
docker-build:
	docker build -f deploy/Dockerfile -t $(IMAGE):$(VERSION) -t $(IMAGE):latest \
	  --build-arg VERSION=$(VERSION) .

## docker-run: run the image locally
docker-run:
	docker run --rm -p 8080:8080 -e LOG_LEVEL=debug $(IMAGE):latest
