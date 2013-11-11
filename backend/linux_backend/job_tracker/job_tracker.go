package job_tracker

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/vito/garden/command_runner"
)

type JobTracker struct {
	runner command_runner.CommandRunner

	jobs      map[uint32]*Job
	nextJobID uint32

	sync.RWMutex
}

type UnknownJobError struct {
	JobID uint32
}

func (e UnknownJobError) Error() string {
	return fmt.Sprintf("unknown job: %d", e.JobID)
}

func New(runner command_runner.CommandRunner) *JobTracker {
	return &JobTracker{
		runner: runner,

		jobs: make(map[uint32]*Job),
	}
}

func (t *JobTracker) Spawn(cmd *exec.Cmd) uint32 {
	job := NewJob(cmd)

	t.Lock()

	jobID := t.nextJobID
	t.nextJobID++

	t.jobs[jobID] = job

	t.Unlock()

	go t.track(jobID, job)

	return jobID
}

func (t *JobTracker) Link(jobID uint32) (uint32, []byte, []byte, error) {
	t.RLock()
	job, ok := t.jobs[jobID]
	t.RUnlock()

	if !ok {
		return 0, nil, nil, UnknownJobError{jobID}
	}

	<-job.Link()

	return job.ExitStatus, job.Stdout.Bytes(), job.Stderr.Bytes(), nil
}

func (t *JobTracker) track(jobID uint32, job *Job) {
	job.Run(t.runner)

	t.Lock()
	delete(t.jobs, jobID)
	t.Unlock()
}
