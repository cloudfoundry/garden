package process_tracker

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"

	"github.com/pivotal-cf-experimental/garden/backend"
	"github.com/pivotal-cf-experimental/garden/command_runner"
)

type Process struct {
	ID uint32

	containerPath string
	runner        command_runner.CommandRunner

	waitingLinks   *sync.Mutex
	completionLock *sync.Mutex
	runningLink    *sync.Once
	link           *exec.Cmd
	unlinked       bool

	streams     []chan backend.ProcessStream
	streamsLock *sync.RWMutex

	completed bool

	exitStatus uint32
	stdout     *namedStream
	stderr     *namedStream
}

func NewProcess(
	id uint32,
	containerPath string,
	runner command_runner.CommandRunner,
) *Process {
	p := &Process{
		ID: id,

		containerPath: containerPath,
		runner:        runner,

		streamsLock: &sync.RWMutex{},

		waitingLinks:   &sync.Mutex{},
		runningLink:    &sync.Once{},
		completionLock: &sync.Mutex{},
	}

	p.stdout = newNamedStream(p, backend.ProcessStreamSourceStdout)
	p.stderr = newNamedStream(p, backend.ProcessStreamSourceStderr)

	return p
}

func (p *Process) Spawn(cmd *exec.Cmd) (ready, active chan error) {
	ready = make(chan error, 1)
	active = make(chan error, 1)

	spawnPath := path.Join(p.containerPath, "bin", "iomux-spawn")
	processDir := path.Join(p.containerPath, "processes", fmt.Sprintf("%d", p.ID))

	mkdir := &exec.Cmd{
		Path: "mkdir",
		Args: []string{"-p", processDir},
	}

	err := p.runner.Run(mkdir)
	if err != nil {
		ready <- err
		return
	}

	spawn := &exec.Cmd{
		Path:  spawnPath,
		Stdin: cmd.Stdin,
	}

	spawn.Args = append([]string{processDir}, cmd.Path)
	spawn.Args = append(spawn.Args, cmd.Args...)

	spawn.Env = cmd.Env

	spawnR, spawnW, err := os.Pipe()
	if err != nil {
		ready <- err
		return
	}

	spawn.Stdout = spawnW

	spawnOut := bufio.NewReader(spawnR)

	err = p.runner.Start(spawn)
	if err != nil {
		ready <- err
		return
	}

	go func() {
		defer func() {
			spawn.Wait()
			spawnW.Close()
			spawnR.Close()
		}()

		_, err = spawnOut.ReadBytes('\n')
		if err != nil {
			ready <- err
			return
		}

		ready <- nil

		_, err = spawnOut.ReadBytes('\n')
		if err != nil {
			active <- err
			return
		}

		active <- nil
	}()

	return
}

func (p *Process) Link() {
	p.waitingLinks.Lock()
	defer p.waitingLinks.Unlock()

	p.runningLink.Do(p.runLinker)
}

func (p *Process) Unlink() error {
	if p.link != nil {
		p.unlinked = true
		return p.runner.Signal(p.link, os.Interrupt)
	}

	return nil
}

func (p *Process) Stream() chan backend.ProcessStream {
	return p.registerStream()
}

func (p *Process) runLinker() {
	linkPath := path.Join(p.containerPath, "bin", "iomux-link")
	processDir := path.Join(p.containerPath, "processes", fmt.Sprintf("%d", p.ID))

	p.link = &exec.Cmd{
		Path:   linkPath,
		Args:   []string{"-w", path.Join(processDir, "cursors"), processDir},
		Stdout: p.stdout,
		Stderr: p.stderr,
	}

	p.runner.Run(p.link)

	if p.unlinked {
		// iomux-link was killed on shutdown via .Unlink; command didn't
		// actually exit, so just block forever until server dies and re-links
		select {}
	}

	exitStatus := uint32(255)

	if p.link.ProcessState != nil {
		exitStatus = uint32(p.link.ProcessState.Sys().(syscall.WaitStatus).ExitStatus())
	}

	p.exitStatus = exitStatus

	p.closeStreams()
}

func (p *Process) registerStream() chan backend.ProcessStream {
	p.streamsLock.Lock()
	defer p.streamsLock.Unlock()

	stream := make(chan backend.ProcessStream, 1000)

	p.streams = append(p.streams, stream)

	if p.completed {
		defer p.closeStreams()
	}

	return stream
}

func (p *Process) sendToStreams(chunk backend.ProcessStream) {
	p.streamsLock.RLock()
	defer p.streamsLock.RUnlock()

	for _, stream := range p.streams {
		select {
		case stream <- chunk:
		default:
		}
	}
}

func (p *Process) closeStreams() {
	p.streamsLock.RLock()
	defer p.streamsLock.RUnlock()

	for _, stream := range p.streams {
		stream <- backend.ProcessStream{ExitStatus: &(p.exitStatus)}
		close(stream)
	}

	p.streams = nil
	p.completed = true
}
