PROGRAMS    = sdk

TARGETBIN		= 0chainwebsdk.wasm
FILESERVERBIN	= fileserver.exe
WEBSDKDIR 		= ./0chainwebsdk
ELECTRONSDKDIR 	= ./0chainelectronsdk
OUTDIR			= $(WEBSDKDIR) $(ELECTRONSDKDIR)

.PHONY: all sdk
all: sdk

$(FILESERVERBIN): fileserver.go
	@echo "Building $(FILESERVERBIN) ..."
	@go build -o $(FILESERVERBIN) fileserver.go

$(OUTDIR):
	@$(eval $(foreach dir, $(OUTDIR), $(shell mkdir -p $(dir))))
	$(eval WASM_PATH=$(shell go env GOROOT))
	$(eval WASM_EXEC=$(WASM_PATH)/misc/wasm/wasm_exec.js)
	@echo "Copying $(WASM_EXEC) to SDK folders..."
	@$(eval $(foreach dir, $(OUTDIR),$(shell cp $(WASM_EXEC) $(dir)/)))

sdk: $(TARGETBIN) $(FILESERVERBIN) $(OUTDIR)
# Copy common files to all SDK dir
	@$(eval $(foreach dir, $(OUTDIR),$(shell cp index.html $(dir)/)))
	@$(eval $(foreach dir, $(OUTDIR),$(shell cp $(TARGETBIN) $(dir)/)))
# Copy Web files
	@$(eval $(shell cp -f $(FILESERVERBIN) $(WEBSDKDIR)))
# Copy electron files
	@$(eval $(shell cp -rf electron/* $(ELECTRONSDKDIR)))

$(TARGETBIN): websdk.go
	@test -d $(GOPATH)/github.com/klauspost/reedsolomon || go get github.com/klauspost/reedsolomon
	@echo "Building $(TARGETBIN) ..."
	@GOARCH=wasm GOOS=js go build -o $(TARGETBIN) websdk.go

clean:
	rm -rf $(TARGETBIN) $(OUTDIR) $(FILESERVERBIN)