package job_tracker

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"
	"syscall"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/command_runner"
)

type Job struct {
	id            uint32
	containerPath string
	cmd           *exec.Cmd
	runner        command_runner.CommandRunner

	waitingLinks *sync.Cond
	runningLink  *sync.Once

	streams    []chan backend.JobStream
	streamLock *sync.RWMutex

	completed bool

	exitStatus uint32
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
}

func NewJob(id uint32, containerPath string, cmd *exec.Cmd, runner command_runner.CommandRunner) *Job {
	return &Job{
		id:            id,
		containerPath: containerPath,
		cmd:           cmd,
		runner:        runner,

		waitingLinks: sync.NewCond(&sync.Mutex{}),
		runningLink:  &sync.Once{},
		streamLock:   &sync.RWMutex{},
	}
}

func (j *Job) Spawn() (ready, active chan error) {
	ready = make(chan error, 1)
	active = make(chan error, 1)

	spawnPath := path.Join(j.containerPath, "bin", "iomux-spawn")
	jobDir := path.Join(j.containerPath, "jobs", fmt.Sprintf("%d", j.id))

	err := os.MkdirAll(jobDir, 0755)
	if err != nil {
		ready <- err
		return
	}

	spawn := exec.Command(spawnPath, jobDir)
	spawn.Args = append(spawn.Args, j.cmd.Args...)
	spawn.Stdin = j.cmd.Stdin

	stdout, err := spawn.StdoutPipe()
	if err != nil {
		ready <- err
		return
	}

	spawnOut := bufio.NewReader(stdout)

	err = j.runner.Start(spawn)
	if err != nil {
		ready <- err
		return
	}

	go func() {
		defer spawn.Wait()

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

func (j *Job) Link() (uint32, []byte, []byte, error) {
	j.waitingLinks.L.Lock()
	defer j.waitingLinks.L.Unlock()

	if j.completed {
		return j.exitStatus, j.stdout.Bytes(), j.stderr.Bytes(), nil
	}

	j.runningLink.Do(j.runLinker)

	if !j.completed {
		j.waitingLinks.Wait()
	}

	return j.exitStatus, j.stdout.Bytes(), j.stderr.Bytes(), nil
}

func (j *Job) Stream() chan backend.JobStream {
	return j.registerStream()
}

func (j *Job) runLinker() {
	linkPath := path.Join(j.containerPath, "bin", "iomux-link")
	jobDir := path.Join(j.containerPath, "jobs", fmt.Sprintf("%d", j.id))

	link := exec.Command(linkPath, jobDir)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	link.Stdout = newNamedStream(j, "stdout", stdout)
	link.Stderr = newNamedStream(j, "stderr", stderr)

	j.runner.Run(link)

	exitStatus := uint32(255)

	if link.ProcessState != nil {
		// TODO: why do I need to modulo this?
		exitStatus = uint32(link.ProcessState.Sys().(syscall.WaitStatus) % 255)
	}

	j.exitStatus = exitStatus
	j.stdout = stdout
	j.stderr = stderr

	j.completed = true

	j.sendToStreams(backend.JobStream{ExitStatus: &exitStatus})
	j.closeStreams()

	j.waitingLinks.Broadcast()
}

func (j *Job) registerStream() chan backend.JobStream {
	j.streamLock.Lock()
	defer j.streamLock.Unlock()

	stream := make(chan backend.JobStream)

	j.streams = append(j.streams, stream)

	return stream
}

func (j *Job) sendToStreams(chunk backend.JobStream) {
	j.streamLock.RLock()
	defer j.streamLock.RUnlock()

	for _, sink := range j.streams {
		sink <- chunk
	}
}

func (j *Job) closeStreams() {
	j.streamLock.RLock()
	defer j.streamLock.RUnlock()

	for _, sink := range j.streams {
		close(sink)
	}
}
