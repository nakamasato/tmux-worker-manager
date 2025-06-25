# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool called `gtw` (git-tmux-workspace) that manages development workflows by integrating:
- Git worktrees for isolated feature development
- tmux sessions with structured pane layouts
- Claude AI integration for development assistance

The tool creates isolated development environments where each "worker" has its own git worktree and tmux session with dedicated panes for main work, git operations, and Claude AI interaction.

## Build and Development Commands

### Core Commands
- `make build` - Build the binary to `bin/gtw`
- `make install-user` - Install to `~/.local/bin` (recommended, no sudo)
- `make install` - Install system-wide to `/usr/local/bin` (requires sudo)
- `make clean` - Remove build artifacts and `.tmux-workers.json`
- `make setup` - Setup development environment and create worktree directory

### Development and Testing
- `make dev ARGS="command"` - Build and run with specified arguments
- `make test` - Run basic functionality tests
- `make status` - Show current workers status
- `make help` - Display all available make targets

### Common Development Workflows
```bash
# Test adding a worker
make dev ARGS="add test-issue"

# Test listing workers
make dev ARGS="list"

# Test removing a worker
make dev ARGS="remove test-issue"
```

## Architecture

### Core Data Structures
- `Worker` struct: Represents a development environment with ID, worktree path, tmux session name, creation time, and status
- `Config` struct: Contains array of workers, persisted to `.tmux-workers.json`

### Key Components

#### Worker Lifecycle (`main.go`)
1. **Creation** (`addWorker`): Creates git worktree → tmux session → pane layout → starts Claude
2. **Management** (`listWorkers`, `showWorkerStatus`): Tracks worker state and tmux session health
3. **Cleanup** (`removeWorker`): Tears down tmux session → removes git worktree → updates config

#### tmux Pane Layout
Each worker creates a 3-pane tmux session:
- Pane 0: Main development work
- Pane 1: Git operations  
- Pane 2: Claude AI interaction

#### Session Naming Convention
- Uses `<project>` format (e.g., current directory name like `my-project`)
- Each worker creates panes within this session rather than separate sessions

### Configuration Management
- Uses `.tmux-workers.json` for persistent worker state
- JSON structure tracks all worker metadata and status
- Automatic cleanup of orphaned sessions/worktrees

### Dependencies
- Go 1.24.3+ (specified in go.mod)
- tmux (for session management)
- git (for worktree operations)
- Claude CLI (optional, with fallback to `npx claude`)

## Important Implementation Details

### Git Worktree Integration
- Creates worktrees in `./worktree/<worker-id>/` directory
- Each worker gets isolated git environment
- Automatic cleanup on worker removal with force fallback

### tmux Session Management
- Detached session creation with working directory set to worktree
- Health checking via `tmux has-session` 
- Automatic status detection (active/inactive)
- Graceful session cleanup on removal

### Claude Integration
- Attempts to find `claude` command in PATH
- Falls back to `npx claude` if not found
- Runs in dedicated pane for AI assistance
- Configurable via `claudeCmd` variable in `addWorker` function

### Error Handling
- Robust cleanup on partial failures (e.g., removes worktree if tmux session creation fails)
- Graceful degradation for missing dependencies
- Status validation for tmux sessions and worktrees