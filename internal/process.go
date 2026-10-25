package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

// ExitStatus carries a child's status to the CLI without exiting inside a library.
type ExitStatus struct{ Code int }

func (e *ExitStatus) Error() string { return "child process exited" }

func ChildExitError(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ExitStatus{Code: processExitCode(exit)}
	}
	return err
}

func NewChildCommand(secrets map[string]string, command string, args []string, clean bool) (*exec.Cmd, error) {
	env, err := ChildEnv(os.Environ(), secrets, clean)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(command, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	configureProcess(cmd)
	return cmd, nil
}

func RunChild(cmd *exec.Cmd) error { return RunChildContext(context.Background(), cmd) }

// RunChildContext forwards shutdown signals to the child group, waits for it to
// be reaped, and escalates to kill after a bounded grace period.
func RunChildContext(ctx context.Context, cmd *exec.Cmd) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, shutdownSignals()...)
	defer signal.Stop(signals)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var timer *time.Timer
	var deadline <-chan time.Time
	canceled := ctx.Done()
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case err := <-done:
			return ChildExitError(err)
		case sig := <-signals:
			signalProcess(cmd, sig)
			if timer == nil {
				timer = time.NewTimer(3 * time.Second)
				deadline = timer.C
			} else {
				killProcess(cmd)
			}
		case <-canceled:
			canceled = nil
			signalProcess(cmd, terminationSignal())
			if timer == nil {
				timer = time.NewTimer(3 * time.Second)
				deadline = timer.C
			}
		case <-deadline:
			killProcess(cmd)
			deadline = nil
		}
	}
}
