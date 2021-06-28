.PHONY: test lint integration-tests

test:
	go test ./...;

lint:
	golangci-lint run --timeout 2m0s;

integration-tests:
	mkdir -p docker.local/blobber1/files/files/exa/mpl/eId/objects/tmp/Mon/Wen
	go test ./... -args integration;
	rm -r docker.local/blobber1/files/files/exa/mpl/eId/objects/tmp/Mon