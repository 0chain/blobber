PROGRAMS    = sdk

OUTDIR		= ./0chainwebsdk

.PHONY: all
all: $(PROGRAMS)

fileserver:
	@echo "Building fileserver ..."
	@go build -o $(OUTDIR)/fileserver fileserver.go

setup:
	@$(eval $(shell mkdir -p $(OUTDIR)))
	@$(eval WASM_PATH=$(shell go env GOROOT))
	@$(eval WASM_EXEC=$(WASM_PATH)/misc/wasm/wasm_exec.js)
	@echo "Copying wasm_exec.js from $(WASM_EXEC) ..."
	@$(eval $(shell cp $(WASM_EXEC) $(OUTDIR)/))
	@$(eval $(shell cp index.html "$(OUTDIR)/"))

sdk: setup
	@echo "Building sdk.wasm ..."
	@GOARCH=wasm GOOS=js go build -o $(OUTDIR)/0chainwebsdk.wasm websdk.go

clean:
	rm -rf $(OUTDIR)