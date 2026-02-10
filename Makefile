.PHONY: help build run test clean docker-up docker-down docker-build lint k6-gateway k6-gateway-100k k6-gateway-mixed k6-gateway-create k6-gateway-redirect k6-gateway-stats k6-gateway-health

help:
	@echo "Available commands:"
	@echo "  make build       - Build the high-TPS application"
	@echo "  make run         - Run the high-TPS application locally"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start containers (app + mongo)"
	@echo "  make docker-down - Stop containers"
	@echo "  make docker-build - Build and start containers"
	@echo "  make lint        - Run golangci-lint (if installed)"
	@echo "  make k6-gateway  - Run k6 mixed workload with local-safe defaults"
	@echo "  make k6-gateway-100k - Run mixed workload with 100k TPS target"
	@echo "  make k6-gateway-create - Run create-only load test (insertions)"
	@echo "  make k6-gateway-redirect - Run redirect-only load test"
	@echo "  make k6-gateway-stats - Run stats-only load test"
	@echo "  make k6-gateway-health - Run health-only load test"

build:
	@echo "Building (high TPS)..."
	@go build -o bin/api ./cmd/api_hightps

run:
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

k6-gateway:
	@LT_MODE=mixed LT_TARGET_TPS=1000 LT_PRE_ALLOCATED_VUS=400 LT_MAX_VUS=4000 LT_MIXED_CREATE_PCT=0 LT_MIXED_REDIRECT_PCT=90 LT_MIXED_STATS_PCT=10 k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-100k:
	@LT_MODE=mixed LT_TARGET_TPS=100000 LT_PRE_ALLOCATED_VUS=$${LT_PRE_ALLOCATED_VUS:-20000} LT_MAX_VUS=$${LT_MAX_VUS:-50000} k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-mixed:
	@LT_MODE=mixed LT_TARGET_TPS=1000 LT_PRE_ALLOCATED_VUS=400 LT_MAX_VUS=4000 k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-create:
	@LT_MODE=create k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-redirect:
	@LT_MODE=redirect k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-stats:
	@LT_MODE=stats k6 run ./tests/k6/api_gateway_tps.js

k6-gateway-health:
	@LT_MODE=health k6 run ./tests/k6/api_gateway_tps.js
