package command_runner

import (
	"os/exec"
)

type CommandRunner interface {
	Run(*exec.Cmd) error
	Start(*exec.Cmd) error
}

type RealCommandRunner struct{}

func New() *RealCommandRunner {
	return &RealCommandRunner{}
}

func (r *RealCommandRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

func (r *RealCommandRunner) Start(cmd *exec.Cmd) error {
	return cmd.Start()
}
