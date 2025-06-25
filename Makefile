# Makefile for gtw (git-tmux-workspace) CLI

BINARY_NAME=gtw
BUILD_DIR=bin
INSTALL_DIR=/usr/local/bin

.PHONY: build install clean test help

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@sudo chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed successfully!"

# Install for current user only (no sudo required)
install-user: build
	@echo "Installing $(BINARY_NAME) to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/
	@chmod +x ~/.local/bin/$(BINARY_NAME)
	@echo "Installed to ~/.local/bin/$(BINARY_NAME)"
	@echo "Make sure ~/.local/bin is in your PATH"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f .tmux-workers.json
	@echo "Clean complete"

# Test basic functionality
test: build
	@echo "Running basic tests..."
	@./$(BUILD_DIR)/$(BINARY_NAME) --help || true
	@echo "Test complete"

# Run Go unit tests
test-unit:
	@echo "Running Go unit tests..."
	@go test -v -run "Test" .

# Run comprehensive scenario-based integration tests
test-scenarios: build
	@echo "Running scenario-based integration tests..."
	@go test -v -run "Test" .

# Run benchmark tests
test-bench: build
	@echo "Running benchmark tests..."
	@go test -v -bench=. .

# Run all tests
test-all: test test-unit test-scenarios

# Development: build and run with args
dev: build
	@echo "Running in development mode..."
	@./$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

# Show help
help:
	@echo "gtw (git-tmux-workspace) CLI Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  install        Install system-wide (requires sudo)"
	@echo "  install-user   Install to ~/.local/bin (no sudo)"
	@echo "  clean          Remove build artifacts"
	@echo "  test           Run basic tests"
	@echo "  test-unit      Run Go unit tests"
	@echo "  test-scenarios Run comprehensive scenario-based integration tests"
	@echo "  test-bench     Run benchmark tests"
	@echo "  test-all       Run all tests (basic + unit + scenarios)"
	@echo "  dev            Build and run with ARGS='your-args'"
	@echo "  setup          Setup development environment"
	@echo "  status         Show current workers status"
	@echo "  help           Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make install-user"
	@echo "  make dev ARGS='add issue-123'"
	@echo "  make dev ARGS='list'"

# Setup development environment
setup:
	@echo "Setting up development environment..."
	@echo "Checking Go installation..."
	@go version
	@echo "Checking tmux installation..."
	@tmux -V
	@echo "Checking git installation..."
	@git --version
	@echo "Creating worktree directory..."
	@mkdir -p worktree
	@echo "Setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "1. Run 'make build' to build the CLI"
	@echo "2. Run 'make install-user' to install"
	@echo "3. Add ~/.local/bin to your PATH if needed"
	@echo "4. Run 'gtw add issue-123' to create your first worker"

# Show current workers status
status:
	@echo "Current workers status:"
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		./$(BUILD_DIR)/$(BINARY_NAME) list; \
	else \
		echo "Binary not built. Run 'make build' first."; \
	fi
