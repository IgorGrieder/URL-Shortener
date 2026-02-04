.PHONY: help build run test clean docker-up docker-down docker-build lint

help:
	@echo "Available commands:"
	@echo "  make build       - Build the application"
	@echo "  make build-hightps - Build the high-TPS binary"
	@echo "  make run         - Run the application locally"
	@echo "  make run-hightps - Run the high-TPS binary"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start containers (app + mongo)"
	@echo "  make docker-down - Stop containers"
	@echo "  make docker-build - Build and start containers"
	@echo "  make lint        - Run golangci-lint (if installed)"

build:
	@echo "Building..."
	@go build -o bin/api ./cmd/api

build-hightps:
	@echo "Building (high TPS)..."
	@go build -o bin/api_hightps ./cmd/api_hightps

run:
	@go run ./cmd/api

run-hightps:
	@go run ./cmd/api_hightps

test:
	@GOCACHE=/tmp/gocache go test -v ./...

clean:
	@rm -rf bin/
	@go clean

docker-up:
	@docker compose up -d

docker-down:
	@docker compose down

docker-build:
	@docker compose up -d --build

lint:
	@golangci-lint run ./...
