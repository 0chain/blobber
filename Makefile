.PHONY: test lint

test:
	cd code/go/0chain.net; go test ./...;

lint:
	cd code/go/0chain.net; golangci-lint run;

integration-tests:
	cd code/go/0chain.net; go test ./... --args integration;