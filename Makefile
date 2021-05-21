.PHONY: test lint integration-tests

test:
	go test ./...;

lint:
	golangci-lint run;