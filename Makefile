.PHONY: test lint

test:
	@for mod_file in $$(find * -name go.mod); do mod_dir=$$(dirname $$mod_file); (cd $$mod_dir; go test ./...); done

lint:
	@for mod_file in $$(find * -name go.mod); do mod_dir=$$(dirname $$mod_file); (cd $$mod_dir; if ! golangci-lint run; then break -1; fi;); done
