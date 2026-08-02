DEFAULT_GOAL := help

.PHONY: help test race coverage lint integration-up integration-test integration-down

help:
	@echo "Usage: make [target]"
	@echo
	@echo "Targets:"
	@echo "  test    Run Go tests (go test -v -cover ./...)"
	@echo "  race    Run all tests with the race detector"
	@echo "  coverage Generate a combined coverage report"
	@echo "  lint    Run golangci-lint if available, otherwise go vet"
	@echo "  integration-test Run Docker-backed Kafka/RabbitMQ/NATS/Redis tests"

test:
	go test -v -cover ./...

race:
	go test -race ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

integration-up:
	docker compose -f docker-compose.integration.yml up -d --wait

integration-test:
	@docker compose -f docker-compose.integration.yml up -d --wait; \
	status=0; \
	go test -tags=integration -count=1 -v ./integration || status=$$?; \
	docker compose -f docker-compose.integration.yml down -v; \
	exit $$status

integration-down:
	docker compose -f docker-compose.integration.yml down -v

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, running 'go vet' instead"; \
		go vet ./...; \
	fi
