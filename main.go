package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type Worker struct {
	ID           string    `json:"id"`
	WorktreePath string    `json:"worktree_path"`
	TmuxSession  string    `json:"tmux_session"`
	WindowIndex  int       `json:"window_index"`
	PaneID       string    `json:"pane_id"`       // Stable pane identifier
	PaneIndex    int       `json:"pane_index"`    // For backwards compatibility
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"` // active, inactive
}

type Config struct {
	Workers         []Worker `json:"workers"`
	InitCommand     string   `json:"init_command,omitempty"`      // Command to execute when worker is created
	WorktreePrefix  string   `json:"worktree_prefix,omitempty"`   // Directory prefix for worktrees (default: "worktree")
	ProjectPath     string   `json:"project_path,omitempty"`      // Directory where session was initialized
}

const configFile = ".tmux-workers.json"

var rootCmd = &cobra.Command{
	Use:   "gtw",
	Short: "Manage tmux workers with git worktrees and Claude",
	Long:  `gtw (git-tmux-workspace) is a CLI tool that creates isolated development environments with git worktrees, tmux sessions, and configurable initialization commands.`,
}

func init() {
	// Init command with flags
	var initCommand string
	var initWorktreePrefix string
	
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize tmux session",
		Long:  "Initialize a new tmux session with configurable initialization command and worktree prefix",
		Run: func(cmd *cobra.Command, args []string) {
			initSession(initCommand, initWorktreePrefix)
		},
	}
	
	initCmd.Flags().StringVar(&initCommand, "command", "", "Default initialization command")
	initCmd.Flags().StringVar(&initWorktreePrefix, "worktree-prefix", "", "Prefix for worktree directories (default: 'worktree')")
	
	rootCmd.AddCommand(initCmd)
	
	// Other commands
	rootCmd.AddCommand(&cobra.Command{
		Use:   "destroy",
		Short: "Destroy tmux session",
		Run:   func(cmd *cobra.Command, args []string) { destroySession() },
	})
	
	addCmd := &cobra.Command{
		Use:   "add <worker-id>",
		Short: "Create a new worker",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { addWorker(args[0]) },
	}
	rootCmd.AddCommand(addCmd)
	
	rootCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all workers",
		Run:   func(cmd *cobra.Command, args []string) { listWorkers() },
	})
	
	removeCmd := &cobra.Command{
		Use:   "remove <worker-id>",
		Short: "Remove a worker",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { removeWorker(args[0]) },
	}
	rootCmd.AddCommand(removeCmd)
	
	statusCmd := &cobra.Command{
		Use:   "status <worker-id>",
		Short: "Show worker status",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { showWorkerStatus(args[0]) },
	}
	rootCmd.AddCommand(statusCmd)
	
	rootCmd.AddCommand(&cobra.Command{
		Use:   "attach",
		Short: "Attach to the tmux session",
		Run:   func(cmd *cobra.Command, args []string) { attachSession() },
	})
	
	rootCmd.AddCommand(&cobra.Command{
		Use:   "detach",
		Short: "Detach from the tmux session",
		Run:   func(cmd *cobra.Command, args []string) { detachSession() },
	})
	
	rootCmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check worktree/pane consistency",
		Run:   func(cmd *cobra.Command, args []string) { checkConsistency() },
	})
	
	rootCmd.AddCommand(&cobra.Command{
		Use:   "repair",
		Short: "Repair worktree/pane inconsistencies",
		Run:   func(cmd *cobra.Command, args []string) { repairInconsistencies() },
	})
	
	// Config command with subcommands
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		Run:   func(cmd *cobra.Command, args []string) { showConfig() },
	}
	
	configSetCmd := &cobra.Command{
		Use:   "set <command>",
		Short: "Set initialization command",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { setConfigCommand(args[0]) },
	}
	
	configGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get initialization command",
		Run:   func(cmd *cobra.Command, args []string) { getConfigCommand() },
	}
	
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	rootCmd.AddCommand(configCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}


func loadConfig() (*Config, error) {
	config := &Config{Workers: []Worker{}}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Initialize with default values
		config.InitCommand = getDefaultInitCommand()
		config.WorktreePrefix = getDefaultWorktreePrefix()
		return config, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	// Ensure init command has default if empty
	if config.InitCommand == "" {
		config.InitCommand = getDefaultInitCommand()
	}

	// Ensure worktree prefix has default if empty
	if config.WorktreePrefix == "" {
		config.WorktreePrefix = getDefaultWorktreePrefix()
	}

	return config, err
}

func getDefaultInitCommand() string {
	return "echo 'Hello, worker!'"
}

func getDefaultWorktreePrefix() string {
	return "worktree"
}

func executeInitCommand(config *Config, worktreePath, paneID string) {
	// Execute initialization command
	if config.InitCommand != "" {
		fmt.Printf("Initializing worker pane %s...\n", paneID)
		
		// Get absolute path to worktree directory
		absWorktreePath, err := filepath.Abs(worktreePath)
		if err != nil {
			absWorktreePath = worktreePath
		}
		
		// Change to worktree directory and execute init command
		command := fmt.Sprintf("cd %s && %s", absWorktreePath, config.InitCommand)
		cmd := exec.Command("tmux", "send-keys", "-t", paneID, command, "Enter")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Worker initialization failed: %v\n", err)
		}
	}
}

func saveConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

func addWorker(id string) {
	// Check if we're currently inside a worktree directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return
	}
	
	// Check if current directory is inside a worktree path
	if strings.Contains(cwd, "/worktree/") {
		fmt.Printf("Error: Cannot create worker from within a worktree directory (%s)\n", cwd)
		fmt.Printf("Please run this command from the project root directory\n")
		return
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	// Check if we're in the correct project directory
	if config.ProjectPath != "" {
		if cwd != config.ProjectPath {
			fmt.Printf("Error: Workers can only be created from the initialized project directory\n")
			fmt.Printf("Expected: %s\n", config.ProjectPath)
			fmt.Printf("Current:  %s\n", cwd)
			fmt.Printf("Please cd to the project directory or run 'gtw init' to reinitialize\n")
			return
		}
	}

	// Check if worker already exists
	for _, worker := range config.Workers {
		if worker.ID == id {
			fmt.Printf("Worker '%s' already exists\n", id)
			return
		}
	}

	fmt.Printf("Creating worker '%s'...\n", id)

	// Create worktree path using configured prefix
	worktreePath := filepath.Join("./"+config.WorktreePrefix, id)

	// Step 1: Create git worktree
	fmt.Printf("Creating git worktree at %s...\n", worktreePath)
	
	// Create worktree with new branch (simpler approach)
	cmd := exec.Command("git", "worktree", "add", "-b", id, worktreePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If branch already exists, try without creating new branch
		fmt.Printf("Branch might exist, trying without -b flag...\n")
		cmd = exec.Command("git", "worktree", "add", worktreePath, id)
		output, err = cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error creating git worktree: %v\n", err)
			fmt.Printf("Git output: %s\n", string(output))
			return
		}
	}

	// Step 2: Check session exists and create window
	sessionName := getSessionName()
	if sessionName == "" {
		exec.Command("git", "worktree", "remove", worktreePath).Run()
		return
	}
	
	// Check if session exists
	cmd = exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() != nil {
		fmt.Printf("Error: Session '%s' does not exist. Run 'gtw init' first.\n", sessionName)
		exec.Command("git", "worktree", "remove", worktreePath).Run()
		return
	}
	
	// Always use window 0
	windowIndex := 0
	windowTarget := fmt.Sprintf("%s:%d", sessionName, windowIndex)
	
	fmt.Printf("Adding pane to window %d in session '%s'...\n", windowIndex, sessionName)
	
	// Step 3: Create a new pane by splitting window 0
	// Try vertical split first, then horizontal if that fails
	cmd = exec.Command("tmux", "split-window", "-v", "-t", windowTarget, "-c", worktreePath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Vertical split failed, trying horizontal split...\n")
		
		// Try horizontal split as fallback
		cmd = exec.Command("tmux", "split-window", "-h", "-t", windowTarget, "-c", worktreePath)
		if err := cmd.Run(); err != nil {
			// Get detailed error information
			output, _ := cmd.CombinedOutput()
			fmt.Printf("Error creating pane (both splits failed): %v\n", err)
			fmt.Printf("Tmux output: %s\n", string(output))
			
			// Check current window size and pane count
			sizeCmd := exec.Command("tmux", "display-message", "-t", windowTarget, "-p", "#{window_width}x#{window_height}")
			if sizeOutput, sizeErr := sizeCmd.Output(); sizeErr == nil {
				fmt.Printf("Current window size: %s", string(sizeOutput))
			}
			
			paneCountCmd := exec.Command("tmux", "list-panes", "-t", windowTarget)
			if paneOutput, paneErr := paneCountCmd.Output(); paneErr == nil {
				paneCount := len(strings.Split(strings.TrimSpace(string(paneOutput)), "\n"))
				fmt.Printf("Current pane count: %d\n", paneCount)
			}
			
			exec.Command("git", "worktree", "remove", worktreePath).Run()
			return
		}
	}
	
	// Get the newly created pane ID and index (the currently active pane after split)
	cmd = exec.Command("tmux", "display-message", "-t", windowTarget, "-p", "#{pane_index}:#{pane_id}")
	paneOutput, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting new pane info: %v\n", err)
		exec.Command("git", "worktree", "remove", worktreePath).Run()
		return
	}
	
	parts := strings.Split(strings.TrimSpace(string(paneOutput)), ":")
	if len(parts) != 2 {
		fmt.Printf("Error parsing pane info: %s\n", string(paneOutput))
		exec.Command("git", "worktree", "remove", worktreePath).Run()
		return
	}
	
	var paneIndexNum int
	fmt.Sscanf(parts[0], "%d", &paneIndexNum)
	paneID := parts[1]
	
	fmt.Printf("Created pane %d (ID: %s), setting up workspace...\n", paneIndexNum, paneID)
	
	// Set pane title using pane ID
	exec.Command("tmux", "select-pane", "-t", paneID, "-T", fmt.Sprintf("%s", id)).Run()
	
	// Focus on the new pane
	exec.Command("tmux", "select-pane", "-t", paneID).Run()

	// Add worker to config
	worker := Worker{
		ID:           id,
		WorktreePath: worktreePath,
		TmuxSession:  sessionName,
		WindowIndex:  windowIndex,
		PaneID:       paneID,
		PaneIndex:    paneIndexNum,
		CreatedAt:    time.Now(),
		Status:       "active",
	}

	config.Workers = append(config.Workers, worker)

	if err := saveConfig(config); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		return
	}

	// Execute initialization command
	executeInitCommand(config, worktreePath, paneID)

	fmt.Printf("Worker '%s' created successfully!\n", id)
	fmt.Printf("Tmux session: %s\n", sessionName)
	fmt.Printf("Worktree path: %s\n", worktreePath)
	fmt.Printf("To attach: tmux attach-session -t %s\n", sessionName)
}

func listWorkers() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if len(config.Workers) == 0 {
		fmt.Println("No workers found")
		return
	}

	fmt.Printf("%-20s %-15s %-30s %-25s %-10s %s\n", "ID", "STATUS", "WORKTREE PATH", "TMUX SESSION", "PANE", "CREATED")
	fmt.Println(strings.Repeat("-", 105))

	for _, worker := range config.Workers {
		// Check if tmux pane is actually running by pane ID
		status := worker.Status
		cmd := exec.Command("tmux", "list-panes", "-t", fmt.Sprintf("%s:%d", worker.TmuxSession, worker.WindowIndex), "-f", fmt.Sprintf("#{==:#{pane_id},%s}", worker.PaneID))
		if err := cmd.Run(); err != nil {
			status = "inactive"
		}

		fmt.Printf("%-20s %-15s %-30s %-25s %-10s %s\n",
			worker.ID,
			status,
			worker.WorktreePath,
			worker.TmuxSession,
			fmt.Sprintf("%s", worker.PaneID),
			worker.CreatedAt.Format("2006-01-02 15:04"))
	}
}

func removeWorker(id string) {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	workerIndex := -1
	var worker Worker

	for i, w := range config.Workers {
		if w.ID == id {
			workerIndex = i
			worker = w
			break
		}
	}

	if workerIndex == -1 {
		fmt.Printf("Worker '%s' not found\n", id)
		return
	}

	fmt.Printf("Removing worker '%s'...\n", id)

	// Kill tmux pane using pane ID
	fmt.Printf("Killing tmux pane '%s' (ID: %s)...\n", worker.ID, worker.PaneID)
	cmd := exec.Command("tmux", "kill-pane", "-t", worker.PaneID)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Could not kill tmux pane: %v\n", err)
	}

	// Remove git worktree
	fmt.Printf("Removing git worktree '%s'...\n", worker.WorktreePath)
	cmd = exec.Command("git", "worktree", "remove", worker.WorktreePath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: Could not remove git worktree: %v\n", err)
		// Try force remove
		exec.Command("git", "worktree", "remove", "--force", worker.WorktreePath).Run()
	}

	// Remove from config
	config.Workers = append(config.Workers[:workerIndex], config.Workers[workerIndex+1:]...)

	if err := saveConfig(config); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		return
	}

	fmt.Printf("Worker '%s' removed successfully!\n", id)
}

func showWorkerStatus(id string) {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	var worker *Worker
	for _, w := range config.Workers {
		if w.ID == id {
			worker = &w
			break
		}
	}

	if worker == nil {
		fmt.Printf("Worker '%s' not found\n", id)
		return
	}

	fmt.Printf("Worker: %s\n", worker.ID)
	fmt.Printf("Created: %s\n", worker.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Worktree: %s\n", worker.WorktreePath)
	fmt.Printf("Tmux Session: %s\n", worker.TmuxSession)
	fmt.Printf("Window Index: %d\n", worker.WindowIndex)
	fmt.Printf("Pane ID: %s\n", worker.PaneID)
	fmt.Printf("Pane Index: %d\n", worker.PaneIndex)

	// Check if tmux pane exists by pane ID
	cmd := exec.Command("tmux", "list-panes", "-t", fmt.Sprintf("%s:%d", worker.TmuxSession, worker.WindowIndex), "-f", fmt.Sprintf("#{==:#{pane_id},%s}", worker.PaneID))
	if err := cmd.Run(); err != nil {
		fmt.Printf("Status: inactive (tmux pane not found)\n")
	} else {
		fmt.Printf("Status: active\n")

		// Show tmux pane info using pane ID
		cmd = exec.Command("tmux", "list-panes", "-t", worker.PaneID, "-F", "#{pane_index}: #{pane_title} (#{pane_current_command}) [#{pane_id}]")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("Pane info:\n%s", string(output))
		}
	}

	// Check if worktree exists
	if _, err := os.Stat(worker.WorktreePath); os.IsNotExist(err) {
		fmt.Printf("Worktree: missing\n")
	} else {
		fmt.Printf("Worktree: exists\n")
	}
}

func getCurrentProjectName() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return "project"
	}
	return filepath.Base(cwd)
}

func getSessionName() string {
	projectName := getCurrentProjectName()
	if projectName == "" {
		return ""
	}
	return projectName
}

func initSession(initCommand, worktreePrefix string) {
	sessionName := getSessionName()
	if sessionName == "" {
		return
	}

	// Check if session already exists
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() == nil {
		fmt.Printf("Session '%s' already exists\n", sessionName)
		return
	}

	fmt.Printf("Creating tmux session '%s'...\n", sessionName)
	// Create new tmux session in detached mode
	cmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error creating tmux session: %v\n", err)
		return
	}

	// Set title for the initial pane (project root)
	projectName := getCurrentProjectName()
	exec.Command("tmux", "select-pane", "-t", sessionName+":0.0", "-T", projectName).Run()

	// Save project path and configuration to config
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Warning: Failed to load config: %v\n", err)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Warning: Failed to get current directory: %v\n", err)
		} else {
			config.ProjectPath = cwd
			
			// Set custom values if provided
			if initCommand != "" {
				config.InitCommand = initCommand
				fmt.Printf("Set initialization command to: %s\n", initCommand)
			}
			if worktreePrefix != "" {
				config.WorktreePrefix = worktreePrefix
				fmt.Printf("Set worktree prefix to: %s\n", worktreePrefix)
			}
			
			if err := saveConfig(config); err != nil {
				fmt.Printf("Warning: Failed to save project configuration: %v\n", err)
			}
		}
	}

	fmt.Printf("Session '%s' created successfully!\n", sessionName)
	fmt.Printf("To attach: tmux attach-session -t %s\n", sessionName)
}

func destroySession() {
	sessionName := getSessionName()
	if sessionName == "" {
		return
	}

	// Check if session exists
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() != nil {
		fmt.Printf("Session '%s' does not exist\n", sessionName)
		return
	}

	fmt.Printf("Destroying tmux session '%s'...\n", sessionName)
	cmd = exec.Command("tmux", "kill-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error destroying tmux session: %v\n", err)
		return
	}

	// Clear project path and workers from config
	config, err := loadConfig()
	if err == nil {
		config.ProjectPath = ""
		config.Workers = []Worker{}
		if err := saveConfig(config); err != nil {
			fmt.Printf("Warning: Failed to clear project configuration: %v\n", err)
		}
	}

	fmt.Printf("Session '%s' destroyed successfully!\n", sessionName)
}

func attachSession() {
	sessionName := getSessionName()
	if sessionName == "" {
		return
	}

	// Check if session exists
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() != nil {
		fmt.Printf("Error: Session '%s' does not exist. Run 'gtw init' first.\n", sessionName)
		return
	}

	// Check if we're already inside a tmux session
	if os.Getenv("TMUX") != "" {
		fmt.Printf("Error: Already inside a tmux session. Use 'tmux switch-client -t %s' instead.\n", sessionName)
		return
	}

	fmt.Printf("Attaching to session '%s'...\n", sessionName)
	// Use syscall.Exec to replace current process with tmux attach
	cmd = exec.Command("tmux", "attach-session", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error attaching to session: %v\n", err)
	}
}

func detachSession() {
	// Check if we're inside a tmux session
	if os.Getenv("TMUX") == "" {
		fmt.Println("Error: Not currently inside a tmux session.")
		return
	}

	fmt.Println("Detaching from tmux session...")
	cmd := exec.Command("tmux", "detach-client")
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error detaching from session: %v\n", err)
	}
}

type InconsistencyType int

const (
	MissingWorktree InconsistencyType = iota
	MissingPane
	OrphanedWorktree
	OrphanedPane
)

type Inconsistency struct {
	Type        InconsistencyType
	WorkerID    string
	Description string
}

func checkConsistency() {
	sessionName := getSessionName()
	if sessionName == "" {
		return
	}

	// Check if session exists
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() != nil {
		fmt.Printf("Error: Session '%s' does not exist. Run 'gtw init' first.\n", sessionName)
		return
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Println("Checking worktree/pane consistency...")
	
	var inconsistencies []Inconsistency

	// Get all panes with IDs and titles
	windowTarget := fmt.Sprintf("%s:0", sessionName)
	cmd = exec.Command("tmux", "list-panes", "-t", windowTarget, "-F", "#{pane_id}:#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error listing panes: %v\n", err)
		return
	}

	// Parse panes - map title to pane ID
	paneMap := make(map[string]string) // title -> pane_id
	projectName := getCurrentProjectName()
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && parts[1] != "" && parts[1] != projectName && !strings.Contains(parts[1], "GX3V2YXM92") {
			paneMap[parts[1]] = parts[0] // title -> pane_id
		}
	}

	// Check workers in config
	for _, worker := range config.Workers {
		// Check if pane exists by title
		if _, exists := paneMap[worker.ID]; !exists {
			inconsistencies = append(inconsistencies, Inconsistency{
				Type:        MissingPane,
				WorkerID:    worker.ID,
				Description: fmt.Sprintf("Worker '%s' has worktree but missing pane", worker.ID),
			})
		}

		// Check if worktree exists
		if _, err := os.Stat(worker.WorktreePath); os.IsNotExist(err) {
			inconsistencies = append(inconsistencies, Inconsistency{
				Type:        MissingWorktree,
				WorkerID:    worker.ID,
				Description: fmt.Sprintf("Worker '%s' has pane but missing worktree", worker.ID),
			})
		}
	}

	// Check for orphaned panes (panes without workers in config)
	configWorkers := make(map[string]bool)
	for _, worker := range config.Workers {
		configWorkers[worker.ID] = true
	}

	for paneTitle := range paneMap {
		if !configWorkers[paneTitle] {
			inconsistencies = append(inconsistencies, Inconsistency{
				Type:        OrphanedPane,
				WorkerID:    paneTitle,
				Description: fmt.Sprintf("Pane '%s' exists but no worker in config", paneTitle),
			})
		}
	}

	// Check for orphaned worktrees
	if worktreeDir, err := os.Open("worktree"); err == nil {
		defer worktreeDir.Close()
		if entries, err := worktreeDir.Readdir(-1); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					workerID := entry.Name()
					if !configWorkers[workerID] {
						inconsistencies = append(inconsistencies, Inconsistency{
							Type:        OrphanedWorktree,
							WorkerID:    workerID,
							Description: fmt.Sprintf("Worktree '%s' exists but no worker in config", workerID),
						})
					}
				}
			}
		}
	}

	// Report results
	if len(inconsistencies) == 0 {
		fmt.Println("✅ No inconsistencies found. All worktrees and panes are in sync.")
		return
	}

	fmt.Printf("❌ Found %d inconsistency(ies):\n\n", len(inconsistencies))
	for i, inc := range inconsistencies {
		fmt.Printf("%d. %s\n", i+1, inc.Description)
	}
	
	fmt.Println("\nRun 'gtw repair' to fix these inconsistencies.")
}

func repairInconsistencies() {
	sessionName := getSessionName()
	if sessionName == "" {
		return
	}

	// Check if session exists
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	if cmd.Run() != nil {
		fmt.Printf("Error: Session '%s' does not exist. Run 'gtw init' first.\n", sessionName)
		return
	}

	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Println("Repairing worktree/pane inconsistencies...")
	
	repairCount := 0

	// Get all panes with IDs and titles
	windowTarget := fmt.Sprintf("%s:0", sessionName)
	cmd = exec.Command("tmux", "list-panes", "-t", windowTarget, "-F", "#{pane_id}:#{pane_title}")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error listing panes: %v\n", err)
		return
	}

	// Parse panes - map title to pane ID
	paneMap := make(map[string]string) // title -> pane_id
	projectName := getCurrentProjectName()
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && parts[1] != "" && parts[1] != projectName && !strings.Contains(parts[1], "GX3V2YXM92") {
			paneMap[parts[1]] = parts[0] // title -> pane_id
		}
	}

	// Repair missing panes for existing workers
	for i, worker := range config.Workers {
		if _, exists := paneMap[worker.ID]; !exists {
			fmt.Printf("🔧 Adding missing pane for worker '%s'...\n", worker.ID)
			
			// Create pane
			cmd = exec.Command("tmux", "split-window", "-v", "-t", windowTarget, "-c", worker.WorktreePath)
			if err := cmd.Run(); err != nil {
				fmt.Printf("❌ Error creating pane: %v\n", err)
				continue
			}
			
			// Get the new pane ID and index
			cmd = exec.Command("tmux", "list-panes", "-t", windowTarget, "-F", "#{pane_index}:#{pane_id}")
			output, err := cmd.Output()
			if err != nil {
				fmt.Printf("❌ Error getting pane info: %v\n", err)
				continue
			}
			
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			newPaneIndex := len(lines) - 1
			lastLine := lines[newPaneIndex]
			parts := strings.Split(lastLine, ":")
			if len(parts) != 2 {
				fmt.Printf("❌ Error parsing pane info: %s\n", lastLine)
				continue
			}
			
			paneIndexNum := newPaneIndex
			newPaneID := parts[1]
			fmt.Sscanf(parts[0], "%d", &paneIndexNum)
			
			// Set pane title using pane ID
			exec.Command("tmux", "select-pane", "-t", newPaneID, "-T", worker.ID).Run()
			
			// Update worker config
			config.Workers[i].PaneIndex = paneIndexNum
			config.Workers[i].PaneID = newPaneID
			
			repairCount++
		}

		// Repair missing worktree
		if _, err := os.Stat(worker.WorktreePath); os.IsNotExist(err) {
			fmt.Printf("🔧 Adding missing worktree for worker '%s'...\n", worker.ID)
			
			// Create worktree
			cmd = exec.Command("git", "worktree", "add", "-b", worker.ID, worker.WorktreePath)
			if err := cmd.Run(); err != nil {
				// Branch might exist, try without -b
				cmd = exec.Command("git", "worktree", "add", worker.WorktreePath, worker.ID)
				if err := cmd.Run(); err != nil {
					fmt.Printf("❌ Error creating worktree: %v\n", err)
					continue
				}
			}
			
			repairCount++
		}
	}

	// Handle orphaned panes (add them to config)
	configWorkers := make(map[string]bool)
	for _, worker := range config.Workers {
		configWorkers[worker.ID] = true
	}

	for paneTitle := range paneMap {
		if !configWorkers[paneTitle] {
			fmt.Printf("🔧 Adding orphaned pane '%s' to config...\n", paneTitle)
			
			worktreePath := filepath.Join("./worktree", paneTitle)
			
			// Create worktree if it doesn't exist
			if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
				cmd = exec.Command("git", "worktree", "add", "-b", paneTitle, worktreePath)
				if err := cmd.Run(); err != nil {
					cmd = exec.Command("git", "worktree", "add", worktreePath, paneTitle)
					if err := cmd.Run(); err != nil {
						fmt.Printf("❌ Error creating worktree for orphaned pane: %v\n", err)
						continue
					}
				}
			}
			
			// Find pane ID and index
			cmd = exec.Command("tmux", "list-panes", "-t", windowTarget, "-F", "#{pane_index}:#{pane_id}:#{pane_title}")
			output, err := cmd.Output()
			if err != nil {
				fmt.Printf("❌ Error finding pane info: %v\n", err)
				continue
			}
			
			paneIndex := -1
			paneID := ""
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 && parts[2] == paneTitle {
					fmt.Sscanf(parts[0], "%d", &paneIndex)
					paneID = parts[1]
					break
				}
			}
			
			if paneIndex >= 0 && paneID != "" {
				// Add to config
				worker := Worker{
					ID:           paneTitle,
					WorktreePath: worktreePath,
					TmuxSession:  sessionName,
					WindowIndex:  0,
					PaneID:       paneID,
					PaneIndex:    paneIndex,
					CreatedAt:    time.Now(),
					Status:       "active",
				}
				config.Workers = append(config.Workers, worker)
				repairCount++
			}
		}
	}

	// Handle orphaned worktrees (remove them or add panes)
	if worktreeDir, err := os.Open("worktree"); err == nil {
		defer worktreeDir.Close()
		if entries, err := worktreeDir.Readdir(-1); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					workerID := entry.Name()
					_, paneExists := paneMap[workerID]
					if !configWorkers[workerID] && !paneExists {
						fmt.Printf("🔧 Removing orphaned worktree '%s'...\n", workerID)
						worktreePath := filepath.Join("worktree", workerID)
						cmd = exec.Command("git", "worktree", "remove", worktreePath)
						if err := cmd.Run(); err != nil {
							exec.Command("git", "worktree", "remove", "--force", worktreePath).Run()
						}
						repairCount++
					}
				}
			}
		}
	}

	// Save updated config
	if err := saveConfig(config); err != nil {
		fmt.Printf("❌ Error saving config: %v\n", err)
		return
	}

	if repairCount == 0 {
		fmt.Println("✅ No repairs needed. All worktrees and panes are already in sync.")
	} else {
		fmt.Printf("✅ Repaired %d inconsistency(ies). All worktrees and panes are now in sync.\n", repairCount)
	}
}

func showConfig() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Println("Current configuration:")
	fmt.Println()
	
	fmt.Printf("  Initialization command: %s\n", config.InitCommand)
	fmt.Printf("  Worktree prefix:        %s\n", config.WorktreePrefix)
	if config.ProjectPath != "" {
		fmt.Printf("  Project path:           %s\n", config.ProjectPath)
	}
	
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gtw config set <command>     Set initialization command")
	fmt.Println("  gtw config get               Get initialization command")
	fmt.Println("  gtw init --command <cmd> --worktree-prefix <prefix>  Initialize with custom settings")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gtw config set 'claude --dangerously-skip-permissions'")
	fmt.Println("  gtw config set 'npx claude'")
	fmt.Println("  gtw config set 'npm run dev'")
	fmt.Println("  gtw init --command 'claude' --worktree-prefix 'work'")
}


func setConfigCommand(command string) {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	config.InitCommand = command

	if err := saveConfig(config); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		return
	}

	fmt.Printf("✅ Set initialization command to: %s\n", command)
}

func getConfigCommand() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if config.InitCommand == "" {
		fmt.Println("No initialization command configured")
	} else {
		fmt.Printf("Current initialization command: %s\n", config.InitCommand)
	}
}
