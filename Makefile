# gopack build helpers.

BUILD_DIR := build
LDFLAGS   := -s -w

.PHONY: all extractor packer test vet clean release-binaries

all: extractor packer

## extractor: build the Windows extractor template (GUI stub)
extractor:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS) -H=windowsgui" -o $(BUILD_DIR)/extractor_template.exe ./cmd/extractor

## packer: build the packer for the host platform
packer:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/gopack ./cmd/packer

test:
	go test ./...

vet:
	go vet ./...

## release-binaries: cross-compile everything that ships in a release
release-binaries:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS) -H=windowsgui" -o $(BUILD_DIR)/extractor_template.exe ./cmd/extractor
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/gopack_windows_amd64.exe ./cmd/packer
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/gopack_linux_amd64 ./cmd/packer
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/gopack_darwin_amd64 ./cmd/packer
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/gopack_darwin_arm64 ./cmd/packer

clean:
	rm -rf $(BUILD_DIR) setup.exe
