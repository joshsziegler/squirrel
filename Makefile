.PHONY: build install test format dependencies pre-commit

build:
	go build .

install:
	go install .

test:
	go test ./...

format:
	gofumpt -w -l .

# Install all development dependencies
dependencies:
	go install mvdan.cc/gofumpt@latest

# Please run before commiting and especially before pushing!
pre-commit: format test build
