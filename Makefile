.PHONY: test lint integration-tests

test:
	go test ./...;

lint:
	golangci-lint run --timeout 2m0s;

integration-tests:
	go test ./... -args integration;
