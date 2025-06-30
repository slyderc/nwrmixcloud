.PHONY: build-windows build-macos build-all clean install

# Build for Windows (AMD64)
build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/mixcloud-updater.exe ./cmd/mixcloud-updater

# Build for macOS Intel (AMD64)
build-macos-intel:
	GOOS=darwin GOARCH=amd64 go build -o bin/mixcloud-updater-macos-intel ./cmd/mixcloud-updater

# Build for macOS Apple Silicon (ARM64)
build-macos-arm:
	GOOS=darwin GOARCH=arm64 go build -o bin/mixcloud-updater-macos-arm ./cmd/mixcloud-updater

# Build universal macOS binary (Intel + Apple Silicon)
build-macos: build-macos-intel build-macos-arm
	lipo -create -output bin/mixcloud-updater-macos bin/mixcloud-updater-macos-intel bin/mixcloud-updater-macos-arm
	rm bin/mixcloud-updater-macos-intel bin/mixcloud-updater-macos-arm

# Build for both platforms
build-all: build-windows build-macos

# Clean build artifacts
clean:
	rm -rf bin/
	mkdir -p bin/

install: clean build-all
	cp bin/mixcloud-updater.exe /Volumes/Myriad/Publish
	cp bin/mixcloud-updater-macos /usr/local/bin

# Default target
all: build-all
