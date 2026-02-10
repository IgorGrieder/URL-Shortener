.PHONY: help build run run-outbox-worker run-click-consumer test clean docker-up docker-down docker-build lint k6-crud k6-crud-smoke

help:
	@echo "Available commands:"
	@echo "  make build       - Build the high-TPS application"
	@echo "  make run         - Run the high-TPS application locally"
	@echo "  make run-outbox-worker - Run outbox to Kafka publisher locally"
	@echo "  make run-click-consumer - Run Kafka click consumer locally"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start containers (app + mongo)"
	@echo "  make docker-down - Stop containers"
	@echo "  make docker-build - Build and start containers"
	@echo "  make lint        - Run golangci-lint (if installed)"
	@echo "  make k6-crud-smoke - Run quick CRUD functional test (k6)"
	@echo "  make k6-crud      - Run CRUD functional test profile (k6)"

build:
	@echo "Building (high TPS)..."
	@go build -o bin/api ./cmd/api_hightps

run:
	@go run ./cmd/api_hightps

run-outbox-worker:
	@go run ./cmd/outbox_worker

run-click-consumer:
	@go run ./cmd/click_consumer

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

k6-crud-smoke:
	@LT_VUS=$${LT_VUS:-1} LT_ITERATIONS=$${LT_ITERATIONS:-5} LT_HTTP_TIMEOUT=$${LT_HTTP_TIMEOUT:-10s} LT_MAX_DURATION=$${LT_MAX_DURATION:-1m} k6 run ./tests/k6/api_gateway_crud.js

k6-crud:
	@LT_VUS=$${LT_VUS:-5} LT_ITERATIONS=$${LT_ITERATIONS:-30} LT_HTTP_TIMEOUT=$${LT_HTTP_TIMEOUT:-10s} LT_MAX_DURATION=$${LT_MAX_DURATION:-2m} k6 run ./tests/k6/api_gateway_crud.js
