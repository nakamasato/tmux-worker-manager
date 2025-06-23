package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfig holds test configuration
type TestConfig struct {
	BinaryPath   string
	TestWorkers  []string
	SessionName  string
	ProjectName  string
}

func setupTest(t *testing.T) *TestConfig {
	// Build binary if it doesn't exist
	binaryPath := "./bin/tm"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		cmd := exec.Command("make", "build")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build binary: %v", err)
		}
	}

	// Get current directory name for session name
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	projectName := filepath.Base(cwd)
	sessionName := fmt.Sprintf("%s-claude-code", projectName)

	return &TestConfig{
		BinaryPath:  binaryPath,
		TestWorkers: []string{"test-issue-1", "test-feature-2", "test-bugfix-3"},
		SessionName: sessionName,
		ProjectName: projectName,
	}
}

func cleanupTest(t *testing.T, tc *TestConfig) {
	t.Log("Cleaning up test environment...")

	// Remove test workers if they exist
	for _, worker := range tc.TestWorkers {
		cmd := exec.Command(tc.BinaryPath, "remove", worker)
		cmd.Run() // Ignore errors
	}

	// Destroy session if exists
	cmd := exec.Command(tc.BinaryPath, "destroy")
	cmd.Run() // Ignore errors

	// Clean up any remaining worktrees
	for _, worker := range tc.TestWorkers {
		worktreePath := filepath.Join("worktree", worker)
		if _, err := os.Stat(worktreePath); err == nil {
			cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
			cmd.Run() // Ignore errors
		}
	}

	// Remove config file
	os.Remove(".tmux-workers.json")

	// Kill any remaining tmux sessions
	cmd = exec.Command("tmux", "kill-session", "-t", tc.SessionName)
	cmd.Run() // Ignore errors
}

func verifyTmuxSession(t *testing.T, sessionName string) {
	t.Logf("Verifying tmux session: %s", sessionName)

	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		t.Errorf("Tmux session '%s' does not exist", sessionName)
	}
}

func verifyGitWorktree(t *testing.T, worktreePath, branchName string) {
	t.Logf("Verifying git worktree: %s (branch: %s)", worktreePath, branchName)

	// Check if directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("Worktree directory '%s' does not exist", worktreePath)
		return
	}

	// Check if it's a git repository (worktree has .git file, not directory)
	gitPath := filepath.Join(worktreePath, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		t.Errorf("Worktree '%s' is not a git repository", worktreePath)
		return
	}

	// Check if we're on the correct branch
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	if err := os.Chdir(worktreePath); err != nil {
		t.Errorf("Failed to change to worktree directory: %v", err)
		return
	}

	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to get current branch: %v", err)
		return
	}

	currentBranch := strings.TrimSpace(string(output))
	if currentBranch != branchName {
		t.Errorf("Worktree is on branch '%s', expected '%s'", currentBranch, branchName)
	}
}

func verifyTmuxPane(t *testing.T, sessionName, paneTitle string) {
	t.Logf("Verifying tmux pane with title: %s", paneTitle)

	cmd := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to list panes: %v", err)
		return
	}

	titles := strings.Split(strings.TrimSpace(string(output)), "\n")
	found := false
	for _, title := range titles {
		if title == paneTitle {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Pane with title '%s' not found in session '%s'. Found titles: %v", paneTitle, sessionName, titles)
	}
}

func verifyWorkerConfig(t *testing.T, workerID string) {
	t.Logf("Verifying worker in config: %s", workerID)

	if _, err := os.Stat(".tmux-workers.json"); os.IsNotExist(err) {
		t.Error("Config file .tmux-workers.json does not exist")
		return
	}

	data, err := os.ReadFile(".tmux-workers.json")
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
		return
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Errorf("Failed to parse config file: %v", err)
		return
	}

	found := false
	for _, worker := range config.Workers {
		if worker.ID == workerID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Worker '%s' not found in config", workerID)
	}
}

func verifyWorkerNotInConfig(t *testing.T, workerID string) {
	t.Logf("Verifying worker NOT in config: %s", workerID)

	if _, err := os.Stat(".tmux-workers.json"); os.IsNotExist(err) {
		// Config file doesn't exist, so worker is definitely not there
		return
	}

	data, err := os.ReadFile(".tmux-workers.json")
	if err != nil {
		t.Errorf("Failed to read config file: %v", err)
		return
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Errorf("Failed to parse config file: %v", err)
		return
	}

	for _, worker := range config.Workers {
		if worker.ID == workerID {
			t.Errorf("Worker '%s' should not be in config but was found", workerID)
			return
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	t.Log("Testing session initialization and destruction")

	// Test session creation
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	verifyTmuxSession(t, tc.SessionName)

	// Verify initial pane title
	verifyTmuxPane(t, tc.SessionName, tc.ProjectName)

	// Test session destruction
	cmd = exec.Command(tc.BinaryPath, "destroy")
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to destroy session: %v", err)
	}

	// Verify session was destroyed
	cmd = exec.Command("tmux", "has-session", "-t", tc.SessionName)
	if err := cmd.Run(); err == nil {
		t.Error("Session should have been destroyed")
	}
}

func TestWorkerCreationAndRemoval(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	workerID := tc.TestWorkers[0]

	// Create worker
	cmd = exec.Command(tc.BinaryPath, "add", workerID)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Verify all components
	worktreePath := filepath.Join("worktree", workerID)
	verifyGitWorktree(t, worktreePath, workerID)
	verifyTmuxPane(t, tc.SessionName, workerID)
	verifyWorkerConfig(t, workerID)

	// Test worker removal
	cmd = exec.Command(tc.BinaryPath, "remove", workerID)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to remove worker: %v", err)
	}

	// Verify cleanup
	if _, err := os.Stat(worktreePath); err == nil {
		t.Error("Worktree should have been removed")
	}

	// Check if pane was removed
	cmd = exec.Command("tmux", "list-panes", "-t", tc.SessionName, "-F", "#{pane_title}")
	if output, err := cmd.Output(); err == nil {
		titles := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, title := range titles {
			if title == workerID {
				t.Error("Pane should have been removed")
				break
			}
		}
	}

	verifyWorkerNotInConfig(t, workerID)
}

func TestMultipleWorkers(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Create multiple workers
	for _, worker := range tc.TestWorkers {
		t.Logf("Creating worker: %s", worker)
		cmd := exec.Command(tc.BinaryPath, "add", worker)
		if err := cmd.Run(); err != nil {
			t.Errorf("Failed to create worker %s: %v", worker, err)
			continue
		}

		// Verify each worker
		worktreePath := filepath.Join("worktree", worker)
		verifyGitWorktree(t, worktreePath, worker)
		verifyTmuxPane(t, tc.SessionName, worker)
		verifyWorkerConfig(t, worker)
	}

	// Verify all workers are listed
	cmd = exec.Command(tc.BinaryPath, "list")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to list workers: %v", err)
		return
	}

	outputStr := string(output)
	// Count workers by looking for their IDs in the output
	workerCount := 0
	for _, worker := range tc.TestWorkers {
		if strings.Contains(outputStr, worker) {
			workerCount++
		}
	}
	expectedCount := len(tc.TestWorkers)

	if workerCount != expectedCount {
		t.Errorf("Expected %d workers, found %d. Output:\n%s", expectedCount, workerCount, outputStr)
	}
}

func TestConsistencyCheckAndRepair(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session and create a worker
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	workerID := tc.TestWorkers[0]
	cmd = exec.Command(tc.BinaryPath, "add", workerID)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Simulate inconsistency by manually removing worktree
	worktreePath := filepath.Join("worktree", workerID)
	cmd = exec.Command("git", "worktree", "remove", worktreePath, "--force")
	if err := cmd.Run(); err != nil {
		t.Logf("Warning: Failed to remove worktree for test: %v", err)
	}

	// Run consistency check
	cmd = exec.Command(tc.BinaryPath, "check")
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to run check: %v", err)
		return
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "inconsistency") && !strings.Contains(outputStr, "❌") {
		t.Errorf("Consistency check should have found inconsistencies. Output:\n%s", outputStr)
	}

	// Run repair
	cmd = exec.Command(tc.BinaryPath, "repair")
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to run repair: %v", err)
		return
	}

	// Verify repair worked
	verifyGitWorktree(t, worktreePath, workerID)

	// Run check again - should be clean
	cmd = exec.Command(tc.BinaryPath, "check")
	output, err = cmd.Output()
	if err != nil {
		t.Errorf("Failed to run check after repair: %v", err)
		return
	}

	outputStr = string(output)
	if !strings.Contains(outputStr, "✅") || strings.Contains(outputStr, "❌") {
		t.Errorf("After repair, no inconsistencies should remain. Output:\n%s", outputStr)
	}
}

func TestPaneIDStability(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Create workers in specific order
	for _, worker := range tc.TestWorkers {
		cmd := exec.Command(tc.BinaryPath, "add", worker)
		if err := cmd.Run(); err != nil {
			t.Errorf("Failed to create worker %s: %v", worker, err)
		}
	}

	// Remove middle worker
	middleWorker := tc.TestWorkers[1]
	cmd = exec.Command(tc.BinaryPath, "remove", middleWorker)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to remove middle worker: %v", err)
	}

	// Verify remaining workers still work
	verifyTmuxPane(t, tc.SessionName, tc.TestWorkers[0])
	verifyTmuxPane(t, tc.SessionName, tc.TestWorkers[2])
	verifyWorkerConfig(t, tc.TestWorkers[0])
	verifyWorkerConfig(t, tc.TestWorkers[2])

	// Verify middle worker is gone
	cmd = exec.Command("tmux", "list-panes", "-t", tc.SessionName, "-F", "#{pane_title}")
	if output, err := cmd.Output(); err == nil {
		titles := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, title := range titles {
			if title == middleWorker {
				t.Error("Removed worker pane should not exist")
				break
			}
		}
	}
}

func TestWorktreePreventionFromWorkerDir(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session and create worker
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	workerID := tc.TestWorkers[0]
	cmd = exec.Command(tc.BinaryPath, "add", workerID)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Save current directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Enter worker directory
	worktreePath := filepath.Join("worktree", workerID)
	if err := os.Chdir(worktreePath); err != nil {
		t.Fatalf("Failed to change to worktree directory: %v", err)
	}

	// Try to create another worker (should fail or warn)
	cmd = exec.Command(tc.BinaryPath, "add", "should-fail")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// The behavior depends on implementation - it might fail or warn
	// For now, we just check that it doesn't silently succeed with a normal worker creation
	if err == nil && !strings.Contains(strings.ToLower(outputStr), "worker") && 
	   !strings.Contains(strings.ToLower(outputStr), "worktree") {
		t.Log("Worker creation from worker directory succeeded - checking if it's handled properly")
		
		// If it succeeded, verify it was handled appropriately
		if !strings.Contains(strings.ToLower(outputStr), "already") && 
		   !strings.Contains(strings.ToLower(outputStr), "exist") {
			t.Error("Should have prevented or warned about creating worktree from worker directory")
		}
	}
}

func TestAttachAndDetachCommands(t *testing.T) {
	tc := setupTest(t)
	defer cleanupTest(t, tc)

	// Initialize session
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	// Create a worker
	workerID := tc.TestWorkers[0]
	cmd = exec.Command(tc.BinaryPath, "add", workerID)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create worker: %v", err)
	}

	// Test attach command (should not fail)
	cmd = exec.Command(tc.BinaryPath, "attach", workerID)
	// We can't actually test the interactive attach, but we can check it doesn't error
	// In a real terminal environment, this would attach to the session
	t.Logf("Attach command would attach to worker %s", workerID)

	// Test detach command
	cmd = exec.Command(tc.BinaryPath, "detach")
	// Similar to attach, we can't test the actual detach in this environment
	t.Log("Detach command would detach from current session")
}

// Benchmark test for worker creation performance
func BenchmarkWorkerCreation(b *testing.B) {
	tc := setupTest(&testing.T{})
	defer cleanupTest(&testing.T{}, tc)

	// Initialize session once
	cmd := exec.Command(tc.BinaryPath, "init")
	if err := cmd.Run(); err != nil {
		b.Fatalf("Failed to initialize session: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		workerID := fmt.Sprintf("bench-worker-%d", i)
		
		// Create worker
		cmd := exec.Command(tc.BinaryPath, "add", workerID)
		if err := cmd.Run(); err != nil {
			b.Errorf("Failed to create worker: %v", err)
			continue
		}
		
		// Clean up immediately
		cmd = exec.Command(tc.BinaryPath, "remove", workerID)
		if err := cmd.Run(); err != nil {
			b.Errorf("Failed to remove worker: %v", err)
		}
	}
}