.PHONY: build run test test-integration lint vet clean

BINARY=gha-tui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/gha-tui

run: build
	./$(BINARY) -R $(REPO)

test:
	go test ./... -v -count=1

test-integration:
	GHA_TUI_INTEGRATION=1 go test ./internal/api/ -v -run Integration

lint: vet
	@echo "Lint passed"

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf /tmp/gha-tui/
