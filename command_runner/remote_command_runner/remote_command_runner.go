package remote_command_runner

import (
	"fmt"
	"os/exec"

	"github.com/vito/garden/command_runner"
)

type RemoteCommandRunner struct {
	username string
	address string
	port uint32

	runner command_runner.CommandRunner
}

func New(username, address string, port uint32, runner command_runner.CommandRunner) *RemoteCommandRunner {
	return &RemoteCommandRunner{
		username: username,
		address: address,
		port: port,

		runner: runner,
	}
}

func (r *RemoteCommandRunner) Run(cmd *exec.Cmd) error {
	return r.runner.Run(r.wrap(cmd))
}

func (r *RemoteCommandRunner) Start(cmd *exec.Cmd) error {
	return r.runner.Start(r.wrap(cmd))
}

func (r *RemoteCommandRunner) Wait(cmd *exec.Cmd) error {
	return r.runner.Wait(cmd)
}

func (r *RemoteCommandRunner) Kill(cmd *exec.Cmd) error {
	return r.runner.Kill(cmd)
}

func (r *RemoteCommandRunner) wrap(cmd *exec.Cmd) *exec.Cmd {
	sshArgs := []string{
		"-l", r.username,
		"-p", fmt.Sprintf("%d", r.port),
		r.address,
	}

	cmd.Args = append([]string{cmd.Path}, cmd.Args...)
	cmd.Args = append(cmd.Env, cmd.Args...)
	cmd.Args = append(sshArgs, cmd.Args...)

	cmd.Path = "ssh"

	cmd.Env = []string{}

	return cmd
}
