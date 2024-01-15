.PHONY: build
build:
	go build -ldflags \
		"-X main.BuildVersion=${shell git describe --tags} \
		 -X main.BuildDate=${shell date -u +%Y.%m.%d}" \
		.

.PHONY: install
install:
	go install -ldflags \
		"-X main.BuildVersion=${shell git describe --tags} \
		 -X main.BuildDate=${shell date -u +%Y.%m.%d}" \
		.

.PHONY: test
test:
	go test ./...

.PHONY: format
format:
	go mod tidy
	gofumpt -w -l .

# Install all development dependencies
.PHONY: install-deps
install-deps:
	go install mvdan.cc/gofumpt@latest

.PHONY: update-deps
update-deps:
	go get -u ./...
	go mod tidy

# Please run before commiting and especially before pushing!
.PHONY: pre-commit
pre-commit: format test build
