package command_runner

import (
	"fmt"
	"os/exec"
)

type CommandRunner interface {
	Run(*exec.Cmd) error
	Start(*exec.Cmd) error
	Wait(*exec.Cmd) error
	Kill(*exec.Cmd) error
}

type RealCommandRunner struct{}

type CommandNotRunningError struct {
	cmd *exec.Cmd
}

func (e CommandNotRunningError) Error() string {
	return fmt.Sprintf("command is not running: %#v", e.cmd)
}

func New() *RealCommandRunner {
	return &RealCommandRunner{}
}

func (r *RealCommandRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

func (r *RealCommandRunner) Start(cmd *exec.Cmd) error {
	return cmd.Start()
}

func (r *RealCommandRunner) Wait(cmd *exec.Cmd) error {
	return cmd.Wait()
}

func (r *RealCommandRunner) Kill(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return CommandNotRunningError{cmd}
	}

	return cmd.Process.Kill()
}
