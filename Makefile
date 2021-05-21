.PHONY: test lint integration-tests

test:
	go test ./...;

lint:
	golangci-lint run;

integration-tests:
	go test ./... --args integration;