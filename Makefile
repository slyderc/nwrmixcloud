.PHONY: build-windows build-macos build-all clean

# Build for Windows (AMD64)
build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/mixcloud-updater.exe ./cmd/mixcloud-updater

# Build for macOS (AMD64)
build-macos:
	GOOS=darwin GOARCH=amd64 go build -o bin/mixcloud-updater-macos ./cmd/mixcloud-updater

# Build for both platforms
build-all: build-windows build-macos

# Clean build artifacts
clean:
	rm -rf bin/
	mkdir -p bin/

# Default target
all: build-all