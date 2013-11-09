package fake_command_runner

import (
	"os/exec"
	"reflect"
)

type FakeCommandRunner struct {
	ExecutedCommands []*exec.Cmd

	commandCallbacks map[*CommandSpec]func() error
}

type CommandSpec struct {
	Path string
	Args []string
	Env  []string
}

func (s CommandSpec) Matches(cmd *exec.Cmd) bool {
	if s.Path != "" && s.Path != cmd.Path {
		return false
	}

	if len(s.Args) > 0 && !reflect.DeepEqual(s.Args, cmd.Args[1:]) {
		return false
	}

	if len(s.Env) > 0 && !reflect.DeepEqual(s.Env, cmd.Env) {
		return false
	}

	return true
}

func New() *FakeCommandRunner {
	return &FakeCommandRunner{
		commandCallbacks: make(map[*CommandSpec]func() error),
	}
}

func (r *FakeCommandRunner) Run(cmd *exec.Cmd) error {
	r.ExecutedCommands = append(r.ExecutedCommands, cmd)

	for spec, callback := range r.commandCallbacks {
		if spec.Matches(cmd) {
			return callback()
		}
	}

	return nil
}

func (r *FakeCommandRunner) WhenRunning(spec CommandSpec, callback func() error) {
	r.commandCallbacks[&spec] = callback
}
