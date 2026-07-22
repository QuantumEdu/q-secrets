package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/QuantumEdu/q-secrets/internal"
)

var runWatch bool

var runCmd = &cobra.Command{
	Use:   "run <project> -- <command> [args...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Run a command with secrets from a project injected as environment variables.

The secrets are decrypted, injected into the child process, and
automatically cleaned up when the process exits.

Examples:
  q-secret run pi -- opencode
  q-secret run pi -- python app.py
  q-secret run pi -- docker-compose up
  q-secret run pi -- printenv ANTHROPIC_KEY

Watch mode (restart on secret changes):
  q-secret run pi --watch -- myapp
`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := args[0]
		command := args[1]
		var cmdArgs []string
		if len(args) > 2 {
			cmdArgs = args[2:]
		}

		if runWatch {
			return runWatchLoop(dbPath, project, command, cmdArgs)
		}

		return runOnce(dbPath, project, command, cmdArgs)
	},
}

// runOnce opens the DB, injects secrets, and runs the command once.
func runOnce(dbPath, project, command string, args []string) error {
	db, err := internal.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	exists, err := db.ProjectExists(project)
	if err != nil {
		return err
	}
	if !exists {
		projs, _ := db.ListProjects()
		if len(projs) == 0 {
			return errors.New("no projects found. Add one with: q-secret add <project> KEY=VALUE")
		}
		return fmt.Errorf("project %q not found. Available projects: %v", project, projs)
	}

	return db.InjectEnv(project, command, args)
}

// runCommandWithEnv builds the environment and starts the child process with
// cancellation support. Returns the command handle so the caller can signal it.
func runCommandWithEnv(ctx context.Context, envMap map[string]string, command string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// watchDBFile watches a file for write/create events and sends a signal on change.
func watchDBFile(ctx context.Context, dbPath string) (<-chan struct{}, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}

	if err := watcher.Add(dbPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watching db file %s: %w (is the DB initialized?)", dbPath, err)
	}

	changed := make(chan struct{}, 1)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					select {
					case changed <- struct{}{}:
					default:
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
			}
		}
	}()

	return changed, nil
}

// runWatchLoop runs the command and restarts it when the DB changes.
func runWatchLoop(dbPath, project, command string, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C / SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Set up the file watcher
	dbChanges, err := watchDBFile(ctx, dbPath)
	if err != nil {
		return err
	}

	// Debounce timer for rapid successive writes
	var debounceTimer *time.Timer

	for {
		db, err := internal.OpenDB(dbPath)
		if err != nil {
			return err
		}

		exists, err := db.ProjectExists(project)
		if err != nil {
			db.Close()
			return err
		}
		if !exists {
			db.Close()
			projs, _ := db.ListProjects()
			if len(projs) == 0 {
				return errors.New("no projects found. Add one with: q-secret add <project> KEY=VALUE")
			}
			return fmt.Errorf("project %q not found. Available projects: %v", project, projs)
		}

		envMap, err := db.BuildEnv(project)
		if err != nil {
			db.Close()
			return err
		}
		db.Close()

		// Start child process with cancellation support
		procCtx, procCancel := context.WithCancel(ctx)
		proc := runCommandWithEnv(procCtx, envMap, command, args)

		if err := proc.Start(); err != nil {
			procCancel()
			return fmt.Errorf("starting command: %w", err)
		}

		// Wait for exit
		procDone := make(chan error, 1)
		go func() {
			procDone <- proc.Wait()
		}()

		select {
		case err := <-procDone:
			procCancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil

		case <-dbChanges:
			// Terminate current process gracefully
			procCancel()
			if err := terminateProcess(proc); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to terminate process: %v\n", err)
			}
			<-procDone

			// Debounce
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(300 * time.Millisecond)

			select {
			case <-debounceTimer.C:
				fmt.Fprintln(os.Stderr, "secrets changed, restarting...")
			case <-ctx.Done():
				return nil
			case <-dbChanges:
				// Drain extra events during debounce
			}

		case <-ctx.Done():
			procCancel()
			terminateProcess(proc)
			<-procDone
			return nil
		}
	}
}

// terminateProcess sends SIGTERM to the process group and falls back to SIGKILL.
func terminateProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// Send to process group so children are also terminated
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		// Fall back to direct signal
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&runWatch, "watch", false, "Watch for secret changes and restart the child process")
}
