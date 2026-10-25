//go:build !windows

package internal

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/term"
)

func configureProcess(cmd *exec.Cmd) {
	// A separate background group cannot read a controlling terminal (SIGTTIN).
	// Interactive children stay in the foreground group; noninteractive jobs
	// get an isolated group so cancellation reaches their descendants.
	if file, ok := cmd.Stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
func shutdownSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP} }
func terminationSignal() os.Signal { return syscall.SIGTERM }
func signalProcess(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process != nil {
		if s, ok := sig.(syscall.Signal); ok {
			if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
				_ = syscall.Kill(-cmd.Process.Pid, s)
			} else {
				_ = cmd.Process.Signal(s)
			}
		}
	}
}
func killProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		if cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
	}
}
func processExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	return err.ExitCode()
}
