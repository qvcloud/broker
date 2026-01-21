DEFAULT_GOAL := help

.PHONY: help test lint

help:
	@echo "Usage: make [target]"
	@echo
	@echo "Targets:"
	@echo "  test    Run Go tests (go test -v -cover ./...)"
	@echo "  lint    Run golangci-lint if available, otherwise go vet"

test:
	go test -v -cover ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, running 'go vet' instead"; \
		go vet ./...; \
	fi
