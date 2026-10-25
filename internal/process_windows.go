//go:build windows

package internal

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
func shutdownSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }
func terminationSignal() os.Signal { return syscall.SIGTERM }

// Windows does not implement POSIX signal forwarding; stop the direct child.
func signalProcess(cmd *exec.Cmd, sig os.Signal) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func killProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func processExitCode(err *exec.ExitError) int { return err.ExitCode() }
