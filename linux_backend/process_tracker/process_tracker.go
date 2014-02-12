package process_tracker

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/pivotal-cf-experimental/garden/backend"
	"github.com/pivotal-cf-experimental/garden/command_runner"
)

type ProcessTracker struct {
	containerPath string
	runner        command_runner.CommandRunner

	processes      map[uint32]*Process
	nextProcessID  uint32
	processesMutex *sync.RWMutex
}

type UnknownProcessError struct {
	ProcessID uint32
}

func (e UnknownProcessError) Error() string {
	return fmt.Sprintf("unknown process: %d", e.ProcessID)
}

func New(containerPath string, runner command_runner.CommandRunner) *ProcessTracker {
	return &ProcessTracker{
		containerPath: containerPath,
		runner:        runner,

		processes:      make(map[uint32]*Process),
		processesMutex: new(sync.RWMutex),
	}
}

func (t *ProcessTracker) Run(cmd *exec.Cmd) (uint32, chan backend.ProcessStream, error) {
	t.processesMutex.Lock()

	processID := t.nextProcessID
	t.nextProcessID++

	process := NewProcess(processID, t.containerPath, t.runner)

	t.processes[processID] = process

	t.processesMutex.Unlock()

	ready, active := process.Spawn(cmd)

	err := <-ready
	if err != nil {
		return 0, nil, err
	}

	processStream := process.Stream()

	go t.link(processID)

	err = <-active
	if err != nil {
		return 0, nil, err
	}

	return processID, processStream, nil
}

func (t *ProcessTracker) Attach(processID uint32) (chan backend.ProcessStream, error) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return nil, UnknownProcessError{processID}
	}

	processStream := process.Stream()

	go t.link(processID)

	return processStream, nil
}

func (t *ProcessTracker) Restore(processID uint32) {
	t.processesMutex.Lock()

	process := NewProcess(processID, t.containerPath, t.runner)

	t.processes[processID] = process

	if processID >= t.nextProcessID {
		t.nextProcessID = processID + 1
	}

	t.processesMutex.Unlock()
}

func (t *ProcessTracker) ActiveProcesses() []*Process {
	t.processesMutex.RLock()
	defer t.processesMutex.RUnlock()

	processes := []*Process{}

	for _, process := range t.processes {
		processes = append(processes, process)
	}

	return processes
}

func (t *ProcessTracker) link(processID uint32) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return
	}

	defer t.unregister(processID)

	process.Link()

	return
}

func (t *ProcessTracker) unregister(processID uint32) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}
