package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/QuantumEdu/q-secrets/internal"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var runWatch bool
var runCleanEnv bool

var runCmd = &cobra.Command{
	Use:   "run <project> -- <command> [args...]",
	Short: "Run a command with secrets injected as environment variables",
	Long: `Run a command with secrets from a project injected as environment variables.

The secrets are decrypted, injected into the child process, and
never include Q_SECRET_KEY. Values exist in child memory until it exits.

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
			return errors.New("no projects found. Add one with: q-secret add <project> KEY --stdin")
		}
		return fmt.Errorf("project %q not found. Available projects: %v", project, projs)
	}

	env, err := db.BuildEnv(project)
	if err != nil {
		return err
	}
	child, err := internal.NewChildCommand(env, command, args, runCleanEnv)
	if err != nil {
		return err
	}
	return internal.RunChild(child)
}

// runCommandWithEnv applies the same environment policy in watch and normal mode.
func runCommandWithEnv(envMap map[string]string, command string, args []string) (*exec.Cmd, error) {
	return internal.NewChildCommand(envMap, command, args, runCleanEnv)
}

// watchDBFile watches a file for write/create events and sends a signal on change.
func watchDBFile(ctx context.Context, dbPath string) (<-chan struct{}, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating file watcher: %w", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		watcher.Close()
		return nil, err
	}
	if err := watcher.Add(filepath.Dir(dbPath)); err != nil {
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
				base := filepath.Base(event.Name)
				relevant := base == filepath.Base(dbPath) || base == filepath.Base(dbPath)+"-wal"
				if relevant && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)) {
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
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	db, err := internal.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	changes, err := watchDBFile(ctx, dbPath)
	if err != nil {
		return err
	}
	env, err := db.BuildEnv(project)
	if err != nil {
		return err
	}
	for {
		childCtx, stop := context.WithCancel(ctx)
		child, err := runCommandWithEnv(env, command, args)
		if err != nil {
			stop()
			return err
		}
		done := make(chan error, 1)
		go func() { done <- internal.RunChildContext(childCtx, child) }()
		restart := false
		for !restart {
			select {
			case err := <-done:
				stop()
				// A parent shutdown must not report success even if the child exits zero.
				select {
				case sig := <-sigCh:
					return shutdownError(sig)
				default:
				}
				return err
			case sig := <-sigCh:
				stop()
				<-done
				return shutdownError(sig)
			case <-changes:
				// WAL writes from other projects and unchanged values must not restart.
				next, err := db.BuildEnv(project)
				if err != nil {
					stop()
					<-done
					return err
				}
				if reflect.DeepEqual(env, next) {
					continue
				}
				stop()
				<-done
				timer := time.NewTimer(300 * time.Millisecond)
				select {
				case <-timer.C:
				case sig := <-sigCh:
					timer.Stop()
					return shutdownError(sig)
				}
				env, err = db.BuildEnv(project)
				if err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "secrets changed, restarting...")
				restart = true
			}
		}
	}
}

func shutdownError(sig os.Signal) error {
	if sig == os.Interrupt {
		return &internal.ExitStatus{Code: 130}
	}
	return &internal.ExitStatus{Code: 143}
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&runWatch, "watch", false, "Watch for secret changes and restart the child process")
	runCmd.Flags().BoolVar(&runCleanEnv, "clean-env", false, "Inherit only PATH and essential Windows OS variables")
	runCmd.Flags().BoolVar(&runCleanEnv, "no-inherit-env", false, "Alias for --clean-env")
}
