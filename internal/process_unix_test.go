//go:build !windows

package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSignaledChildExitCode(t *testing.T) {
	child, err := NewChildCommand(nil, "sh", []string{"-c", "kill -TERM $$"}, true)
	if err != nil {
		t.Fatal(err)
	}
	var status *ExitStatus
	if err := RunChild(child); !errors.As(err, &status) || status.Code != 143 {
		t.Fatalf("signal exit: %v", err)
	}
}

func TestCancellationEscalatesAndReaps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := filepath.Join(t.TempDir(), "ready")
	child, err := NewChildCommand(map[string]string{"READY": ready}, "sh", []string{"-c", `trap '' TERM; printf ready > "$READY"; while :; do sleep 1; done`}, true)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- RunChildContext(ctx, child) }()
	until := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(until) {
			cancel()
			t.Fatal("child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		var status *ExitStatus
		if !errors.As(err, &status) || status.Code != 137 {
			t.Fatalf("expected SIGKILL after grace: %v", err)
		}
		if child.ProcessState == nil {
			t.Fatal("child was not reaped")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("cancellation hung")
	}
}

func TestParentSignalForwarding(t *testing.T) {
	if os.Getenv("Q_SECRETS_SIGNAL_HELPER") == "1" {
		child, err := NewChildCommand(nil, "sh", []string{"-c", `printf ready; exec sleep 30`}, false)
		if err != nil {
			os.Exit(91)
		}
		err = RunChild(child)
		var status *ExitStatus
		if errors.As(err, &status) {
			os.Exit(status.Code)
		}
		os.Exit(92)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := exec.Command(exe, "-test.run=^TestParentSignalForwarding$")
	runner.Env = append(os.Environ(), "Q_SECRETS_SIGNAL_HELPER=1")
	pipe, err := runner.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	defer runner.Process.Kill()
	ready := make(chan error, 1)
	go func() { buf := make([]byte, 5); _, err := pipe.Read(buf); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper not ready")
	}
	if err := runner.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- runner.Wait() }()
	select {
	case err := <-done:
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 143 {
			t.Fatalf("forwarded signal exit: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("signal was not forwarded")
	}
}
