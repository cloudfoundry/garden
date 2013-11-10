package command_runner

import (
	"log"
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("failed to execute %#v:\n\t%s\n", cmd, string(out))
		return err
	}

	return nil
}
