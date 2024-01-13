.PHONY: build install test format dependencies pre-commit

build:
	go build -ldflags \
		"-X main.BuildVersion=${shell git describe --tags} \
		 -X main.BuildDate=${shell date -u +%Y.%m.%d}" \
		.

install:
	go install -ldflags \
		"-X main.BuildVersion=${shell git describe --tags} \
		 -X main.BuildDate=${shell date -u +%Y.%m.%d}" \
		.

test:
	go test ./...

format:
	go mod tidy
	gofumpt -w -l .

# Install all development dependencies
dependencies:
	go install mvdan.cc/gofumpt@latest

# Please run before commiting and especially before pushing!
pre-commit: format test build
