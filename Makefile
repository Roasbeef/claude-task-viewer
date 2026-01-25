PKG := github.com/roasbeef/claude-task-viewer
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags "-X main.Commit=$(COMMIT)"

GO_BIN := $(shell go env GOPATH)/bin
GOIMPORTS_BIN := $(GO_BIN)/gosimports
GOLANGCI_LINT_BIN := $(GO_BIN)/golangci-lint

GOFILES := $(shell find . -name '*.go' -not -path "./vendor/*")

.PHONY: all
all: build

.PHONY: build
build:
	@echo "Building taskviewerd..."
	go build $(LDFLAGS) -o taskviewerd ./cmd/taskviewerd

.PHONY: install
install:
	@echo "Installing taskviewerd..."
	go install $(LDFLAGS) ./cmd/taskviewerd

.PHONY: run
run: build
	@echo "Running taskviewerd..."
	./taskviewerd

.PHONY: dev
dev: build
	@echo "Starting dev server on :8080..."
	@pkill -f taskviewerd 2>/dev/null || true
	@./taskviewerd --listen=:8080 &
	@echo "Server started at http://localhost:8080"

.PHONY: restart
restart: build
	@echo "Restarting server..."
	@pkill -f taskviewerd 2>/dev/null || true
	@sleep 1
	@./taskviewerd --listen=:8080 &
	@echo "Server restarted at http://localhost:8080"

.PHONY: stop
stop:
	@echo "Stopping server..."
	@pkill -f taskviewerd 2>/dev/null || true
	@echo "Server stopped"

.PHONY: logs
logs:
	@echo "Server output (if running in background):"
	@ps aux | grep taskviewerd | grep -v grep || echo "Server not running"

.PHONY: test
test:
	@echo "Running tests..."
	go test -v -race ./...

.PHONY: cover
cover:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html

.PHONY: lint
lint:
	@echo "Running linter..."
	$(GOLANGCI_LINT_BIN) run --timeout 5m

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOIMPORTS_BIN) -w $(GOFILES)

.PHONY: tidy
tidy:
	@echo "Tidying modules..."
	go mod tidy

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f taskviewerd
	rm -f coverage.out coverage.html

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build    - Build the taskviewerd binary"
	@echo "  install  - Install taskviewerd to GOPATH/bin"
	@echo "  run      - Build and run taskviewerd"
	@echo "  dev      - Build and start dev server on :8080 (background)"
	@echo "  restart  - Rebuild and restart dev server"
	@echo "  stop     - Stop the dev server"
	@echo "  logs     - Show if server is running"
	@echo "  test     - Run tests"
	@echo "  cover    - Run tests with coverage"
	@echo "  lint     - Run golangci-lint"
	@echo "  fmt      - Format code with gosimports"
	@echo "  tidy     - Run go mod tidy"
	@echo "  clean    - Remove build artifacts"
