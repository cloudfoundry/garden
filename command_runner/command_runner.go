package command_runner

import (
	"os/exec"
)

type CommandRunner interface {
	Run(*exec.Cmd) error
}

type RealCommandRunner struct{}

func New() *RealCommandRunner {
	return &RealCommandRunner{}
}

func (r *RealCommandRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}
