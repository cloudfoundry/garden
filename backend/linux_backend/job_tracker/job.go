package job_tracker

import (
	"bytes"
	"os/exec"
	"sync"
	"syscall"

	"github.com/vito/garden/command_runner"
)

type Job struct {
	cmd *exec.Cmd

	ExitStatus uint32
	Stdout     *bytes.Buffer
	Stderr     *bytes.Buffer

	links []chan bool

	completed bool

	sync.Mutex
}

func NewJob(cmd *exec.Cmd) *Job {
	return &Job{
		cmd: cmd,

		Stdout: new(bytes.Buffer),
		Stderr: new(bytes.Buffer),
	}
}

func (j *Job) Link() chan bool {
	j.Lock()
	defer j.Unlock()

	if j.completed {
		link := make(chan bool, 1)
		link <- true
		return link
	}

	link := make(chan bool)

	j.links = append(j.links, link)

	return link
}

func (j *Job) Run(runner command_runner.CommandRunner) {
	j.cmd.Stdout = j.Stdout
	j.cmd.Stderr = j.Stderr

	runner.Run(j.cmd)

	j.Lock()

	j.completed = true
	links := j.links

	if j.cmd.ProcessState != nil {
		// TODO: why do I need to modulo this?
		j.ExitStatus = uint32(j.cmd.ProcessState.Sys().(syscall.WaitStatus) % 255)
	}

	j.Unlock()

	for _, link := range links {
		link <- true
	}
}
