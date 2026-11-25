//go:build windows

package main

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// Kill only the process tree created for this invocation, including Chrome.
func stopBridgeProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	killer := exec.CommandContext(ctx, "taskkill.exe", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
	killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := killer.Run(); err != nil {
		return command.Process.Kill()
	}
	return nil
}
