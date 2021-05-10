.PHONY: test lint

test:
	go test ./...;

lint:
	golangci-lint run;