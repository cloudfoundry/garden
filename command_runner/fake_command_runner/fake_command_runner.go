package fake_command_runner

import (
	"os/exec"
	"reflect"

	"sync"
)

type FakeCommandRunner struct {
	executedCommands []*exec.Cmd

	commandCallbacks map[*CommandSpec]func(*exec.Cmd) error

	sync.RWMutex
}

type CommandSpec struct {
	Path  string
	Args  []string
	Env   []string
	Stdin string
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

	if s.Stdin != "" {
		if cmd.Stdin == nil {
			return false
		}

		in := make([]byte, len(s.Stdin))
		_, err := cmd.Stdin.Read(in)
		if err != nil {
			return false
		}

		if string(in) != s.Stdin {
			return false
		}
	}

	return true
}

func New() *FakeCommandRunner {
	return &FakeCommandRunner{
		commandCallbacks: make(map[*CommandSpec]func(*exec.Cmd) error),
	}
}

func (r *FakeCommandRunner) Run(cmd *exec.Cmd) error {
	r.RLock()
	callbacks := r.commandCallbacks
	r.RUnlock()

	r.Lock()
	r.executedCommands = append(r.executedCommands, cmd)
	r.Unlock()

	for spec, callback := range callbacks {
		if spec.Matches(cmd) {
			return callback(cmd)
		}
	}

	return nil
}

func (r *FakeCommandRunner) WhenRunning(spec CommandSpec, callback func(*exec.Cmd) error) {
	r.Lock()
	defer r.Unlock()

	r.commandCallbacks[&spec] = callback
}

func (r *FakeCommandRunner) ExecutedCommands() []*exec.Cmd {
	r.RLock()
	defer r.RUnlock()

	return r.executedCommands
}
