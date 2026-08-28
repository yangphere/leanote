//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// setChildProcessGroup puts the child into its own process group so the
// whole tree (bash → npm → playwright → browsers) can be signalled at once.
func setChildProcessGroup(command *exec.Cmd) error {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	return nil
}

func startChild(command *exec.Cmd) error {
	return command.Start()
}

// killProcessTree signals the child's whole process group with SIGKILL.
func killProcessTree(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil {
		// Fall back to the direct process when the group is already gone.
		if killErr := command.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("kill child pid %d: %w", command.Process.Pid, err)
		}
	}
	return nil
}
