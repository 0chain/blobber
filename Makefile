.PHONY: test lint

test:
	@for mod_file in $$(find * -name go.mod); do mod_dir=$$(dirname $$mod_file); (cd $$mod_dir; go test ./...); done

lint:
	@for mod_file in $$(find * -name go.mod); do mod_dir=$$(dirname $$mod_file); (cd $$mod_dir; go mod tidy; if ! golangci-lint run; then exit 1; fi;); done
