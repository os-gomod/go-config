# =============================================================================
# Go Config Library - Makefile
# Version: 1.0.0
# =============================================================================

# Default environment file
ENV_FILE ?= .env

# Go related variables
GO = go
GO_TEST = $(GO) test
GO_MOD = $(GO) mod
GO_BUILD = $(GO) build
GO_LINT = golangci-lint
GO_BENCH = $(GO) test -bench=.
GO_RACE = $(GO) test -race
GO_COVER = $(GO) test -cover

# Project variables
PROJECT_NAME = config
MODULE_PATH = github.com/os-gomod/go-config
BINARY_NAME = config-example
BIN_DIR = bin
COVERAGE_FILE = coverage.out
COVERAGE_HTML = coverage.html
BENCH_FILE = bench.txt

# Test coverage threshold
COVERAGE_THRESHOLD = 80

# Benchmark parameters
BENCH_TIME = 1s
BENCH_COUNT = 5

# Linter configuration
LINT_CONFIG = .golangci.yml

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: help \
        test test-race test-v test-integration \
        benchmark benchmark-cpu benchmark-mem \
        lint lint-fix fmt vet \
        coverage coverage-html coverage-ci \
        deps deps-update deps-check \
        build build-all cross-build \
        generate docs \
        dev-tools install-tools \
        health prune stats \
        release release-patch release-minor release-major \
        validate

# =============================================================================
# HELP
# =============================================================================
help:
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                     Go Config Library - Makefile                 ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "📦 Development"
	@echo "  make deps             - Download dependencies"
	@echo "  make deps-update      - Update dependencies"
	@echo "  make deps-check       - Check for outdated dependencies"
	@echo "  make fmt              - Format code"
	@echo "  make lint             - Run linter"
	@echo "  make lint-fix         - Run linter and fix issues"
	@echo "  make vet              - Run go vet"
	@echo ""
	@echo "🧪 Testing"
	@echo "  make test             - Run unit tests"
	@echo "  make test-race        - Run tests with race detector"
	@echo "  make test-v           - Run tests verbosely"
	@echo "  make test-integration - Run integration tests"
	@echo "  make benchmark        - Run benchmarks"
	@echo "  make benchmark-cpu    - Run CPU profiling benchmarks"
	@echo "  make benchmark-mem    - Run memory profiling benchmarks"
	@echo "  make coverage         - Generate coverage report"
	@echo "  make coverage-html    - Generate HTML coverage report"
	@echo ""
	@echo "🏗️ Building"
	@echo "  make build            - Build main binary"
	@echo "  make build-all        - Build all packages"
	@echo "  make cross-build      - Cross-compile for multiple platforms"
	@echo "  make generate         - Generate code (mocks, etc.)"
	@echo ""
	@echo "📚 Documentation"
	@echo "  make docs             - Generate documentation"
	@echo "  make docs-serve       - Serve documentation locally"
	@echo ""
	@echo "🔧 Tools"
	@echo "  make dev-tools        - Install development tools"
	@echo "  make install-tools    - Install all required tools"
	@echo "  make validate         - Run all validation checks"
	@echo ""
	@echo "🚀 Release"
	@echo "  make release          - Create a release (requires semver)"
	@echo "  make release-patch    - Create patch release"
	@echo "  make release-minor    - Create minor release"
	@echo "  make release-major    - Create major release"
	@echo ""


# =============================================================================
# DEVELOPMENT
# =============================================================================
fmt:
	@echo "🎨 Formatting code..."
	$(GO) fmt ./...
	@if command -v goimports > /dev/null; then \
		goimports -w -local $(MODULE_PATH) .; \
		echo "✅ Code formatted with goimports"; \
	else \
		echo "⚠️  goimports not installed, using go fmt only"; \
	fi

lint:
	@echo "🔍 Running linter..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG); \
	else \
		$(GO_LINT) run; \
	fi

lint-fix:
	@echo "🔧 Running linter with fixes..."
	@if [ -f "$(LINT_CONFIG)" ]; then \
		$(GO_LINT) run -c $(LINT_CONFIG) --fix; \
	else \
		$(GO_LINT) run --fix; \
	fi

vet:
	@echo "🔬 Running go vet..."
	$(GO) vet ./...
	@echo "✅ Vet completed"

# =============================================================================
# DEPENDENCIES
# =============================================================================
deps:
	@echo "📦 Downloading dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy -v
	@echo "✅ Dependencies downloaded"

deps-update:
	@echo "🔄 Updating dependencies..."
	$(GO_MOD) download
	$(GO_MOD) tidy -v
	$(GO_MOD) verify
	@echo "✅ Dependencies updated"

deps-check:
	@echo "🔍 Checking for outdated dependencies..."
	@if command -v goose > /dev/null; then \
		go list -u -m -f '{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all; \
	else \
		echo "⚠️  goose not installed. Install with: go install github.com/pressly/goose/v3/cmd/goose@latest"; \
	fi

# =============================================================================
# TESTING
# =============================================================================
test:
	@echo "🧪 Running unit tests..."
	$(GO_TEST) -v -race -coverprofile=$(COVERAGE_FILE) ./...

test-race:
	@echo "🏃 Running tests with race detector..."
	$(GO_RACE) -v ./...

test-v:
	@echo "🔊 Running tests verbosely..."
	$(GO_TEST) -v ./...

test-integration:
	@echo "🔗 Running integration tests..."
	@if [ -f "$(ENV_FILE)" ]; then \
		export $$(cat $(ENV_FILE) | xargs); \
	fi
	$(GO_TEST) -v -tags=integration ./...

benchmark:
	@echo "⚡ Running benchmarks..."
	$(GO_BENCH) -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) ./... | tee $(BENCH_FILE)
	@echo "✅ Benchmarks saved to $(BENCH_FILE)"

benchmark-cpu:
	@echo "💻 Running CPU profiling benchmarks..."
	$(GO) test -bench=. -benchtime=$(BENCH_TIME) -cpuprofile=cpu.prof ./...

benchmark-mem:
	@echo "🧠 Running memory profiling benchmarks..."
	$(GO) test -bench=. -benchtime=$(BENCH_TIME) -memprofile=mem.prof ./...

# =============================================================================
# COVERAGE
# =============================================================================
coverage: test
	@echo "📊 Generating coverage report..."
	$(GO) tool cover -func=$(COVERAGE_FILE)
	@coverage=$$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "📈 Coverage: $$coverage%"; \
	if [ $$coverage -lt $(COVERAGE_THRESHOLD) ]; then \
		echo "❌ Error: Coverage $$coverage% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	fi

coverage-html: coverage
	@echo "🌐 Generating HTML coverage report..."
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ HTML report generated: $(COVERAGE_HTML)"
	@if command -v open > /dev/null; then \
		open $(COVERAGE_HTML); \
	elif command -v xdg-open > /dev/null; then \
		xdg-open $(COVERAGE_HTML); \
	fi

coverage-ci: test
	@echo "🔍 Running coverage for CI..."
	$(GO) tool cover -func=$(COVERAGE_FILE)

# =============================================================================
# BUILDING
# =============================================================================
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/example
	@echo "✅ Binary built: $(BIN_DIR)/$(BINARY_NAME)"

build-all:
	@echo "🔨 Building all packages..."
	$(GO_BUILD) ./...

cross-build:
	@echo "🌍 Cross-compiling for multiple platforms..."
	@mkdir -p $(BIN_DIR)/dist
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then \
				ext=".exe"; \
			else \
				ext=""; \
			fi; \
			echo "Building for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch $(GO_BUILD) -o $(BIN_DIR)/dist/$(BINARY_NAME)-$$os-$$arch$$ext ./cmd/example; \
		done; \
	done
	@echo "✅ Cross-compilation complete"

# =============================================================================
# CODE GENERATION
# =============================================================================
generate:
	@echo "⚙️  Generating code..."
	$(GO) generate ./...
	@echo "✅ Code generation complete"

# # =============================================================================
# # DOCUMENTATION
# # =============================================================================
# docs:
# 	@echo "📚 Generating documentation..."
# 	@if command -v godoc > /dev/null; then \
# 		godoc -http=:6060 & \
# 		DOC_PID=$$!; \
# 		sleep 2; \
# 		echo "📖 Documentation available at http://localhost:6060/pkg/$(MODULE_PATH)/"; \
# 		echo "Press Ctrl+C to stop"; \
# 		wait $$DOC_PID; \
# 	else \
# 		echo "⚠️  godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
# 	fi

docs-serve:
	@echo "🌐 Serving documentation..."
	godoc -http=:6060

# =============================================================================
# TOOLS INSTALLATION
# =============================================================================
dev-tools:
	@echo "🔧 Installing development tools..."
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Installing goimports..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing mockery..."
	go install github.com/vektra/mockery/v2@latest
	@echo "Installing godoc..."
	go install golang.org/x/tools/cmd/godoc@latest
	@echo "Installing goose..."
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "✅ Development tools installed"

install-tools: dev-tools
	@echo "📦 Installing additional tools..."
	@echo "Instacing richgo for better test output..."
	go install github.com/kyoh86/richgo@latest
	@echo "Installing gocov for coverage..."
	go install github.com/axw/gocov/gocov@latest
	go install github.com/AlekSi/gocov-xml@latest
	@echo "✅ All tools installed"

# =============================================================================
# VALIDATION
# =============================================================================
validate: deps fmt lint vet test coverage
	@echo "✅ All validation checks passed!"

# =============================================================================
# RELEASE
# =============================================================================
release:
	@echo "🚀 Creating release..."
	@if ! command -v goreleaser > /dev/null; then \
		echo "❌ goreleaser not installed. Install with: go install github.com/goreleaser/goreleaser@latest"; \
		exit 1; \
	fi
	@if [ -z "$$VERSION" ]; then \
		echo "❌ VERSION variable not set. Usage: VERSION=v1.0.0 make release"; \
		exit 1; \
	fi
	goreleaser release --clean

release-patch:
	@echo "📦 Creating patch release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump patch); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Patch release v$$VERSION created"; \
	else \
		echo "❌ semver not installed. Install with: go install github.com/bryanftsg/semver@latest"; \
	fi

release-minor:
	@echo "🔄 Creating minor release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump minor); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Minor release v$$VERSION created"; \
	else \
		echo "❌ semver not installed"; \
	fi

release-major:
	@echo "🚀 Creating major release..."
	@if command -v semver > /dev/null; then \
		VERSION=$$(semver bump major); \
		git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
		git push origin "v$$VERSION"; \
		echo "✅ Major release v$$VERSION created"; \
	else \
		echo "❌ semver not installed"; \
	fi

# =============================================================================
# CLEANUP
# =============================================================================
clean:
	@echo "🧹 Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML) $(BENCH_FILE)
	rm -f *.prof *.test
	rm -rf dist/
	@echo "✅ Cleanup complete"

# =============================================================================
# DEFAULT TARGET
# =============================================================================
.DEFAULT_GOAL := help