package linux_backend

import (
	"bytes"
	"os/exec"
	"path"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend/job_tracker"
	"github.com/vito/garden/command_runner"
)

type LinuxContainer struct {
	id   string
	path string

	spec backend.ContainerSpec

	runner command_runner.CommandRunner

	jobTracker *job_tracker.JobTracker
}

func NewLinuxContainer(id, path string, spec backend.ContainerSpec, runner command_runner.CommandRunner) *LinuxContainer {
	return &LinuxContainer{
		id:   id,
		path: path,

		spec: spec,

		runner: runner,

		jobTracker: job_tracker.New(runner),
	}
}

func (c *LinuxContainer) ID() string {
	return c.id
}

func (c *LinuxContainer) Handle() string {
	if c.spec.Handle != "" {
		return c.spec.Handle
	}

	return c.ID()
}

func (c *LinuxContainer) Start() error {
	start := exec.Command(path.Join(c.path, "start.sh"))

	start.Env = []string{
		"id=" + c.id,
		"container_iface_mtu=1500",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	return c.runner.Run(start)
}

func (c *LinuxContainer) Stop(kill bool) error {
	stop := exec.Command(path.Join(c.path, "stop.sh"))

	if kill {
		stop.Args = append(stop.Args, "-w", "0")
	}

	err := c.runner.Run(stop)
	if err != nil {
		return err
	}

	return nil
}

func (c *LinuxContainer) Info() (backend.ContainerInfo, error) {
	return backend.ContainerInfo{}, nil
}

func (c *LinuxContainer) CopyIn(src, dst string) error {
	return c.rsync(src, "vcap@container:"+dst)
}

func (c *LinuxContainer) CopyOut(src, dst, owner string) error {
	err := c.rsync("vcap@container:"+src, dst)
	if err != nil {
		return err
	}

	if owner != "" {
		chown := exec.Command("chown", "-R", owner, dst)

		err := c.runner.Run(chown)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *LinuxContainer) LimitBandwidth(backend.BandwidthLimits) (backend.BandwidthLimits, error) {
	return backend.BandwidthLimits{}, nil
}

func (c *LinuxContainer) LimitDisk(backend.DiskLimits) (backend.DiskLimits, error) {
	return backend.DiskLimits{}, nil
}

func (c *LinuxContainer) LimitMemory(backend.MemoryLimits) (backend.MemoryLimits, error) {
	return backend.MemoryLimits{}, nil
}

func (c *LinuxContainer) Spawn(spec backend.JobSpec) (uint32, error) {
	wshPath := path.Join(c.path, "bin", "wsh")
	sockPath := path.Join(c.path, "run", "wshd.sock")

	user := "vcap"
	if spec.Privileged {
		user = "root"
	}

	wsh := exec.Command(wshPath, "--socket", sockPath, "--user", user, "/bin/bash")

	wsh.Stdin = bytes.NewBufferString(spec.Script)

	jobID := c.jobTracker.Spawn(wsh)

	return jobID, nil
}

func (c *LinuxContainer) Stream(uint32) (<-chan backend.JobStream, error) {
	return nil, nil
}

func (c *LinuxContainer) Link(jobID uint32) (backend.JobResult, error) {
	exitStatus, stdout, stderr, err := c.jobTracker.Link(jobID)
	if err != nil {
		return backend.JobResult{}, err
	}

	return backend.JobResult{
		ExitStatus: exitStatus,
		Stdout:     stdout,
		Stderr:     stderr,
	}, nil
}

func (c *LinuxContainer) Run(backend.JobSpec) (backend.JobResult, error) {
	return backend.JobResult{}, nil
}

func (c *LinuxContainer) NetIn(uint32, uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (c *LinuxContainer) NetOut(string, uint32) error {
	return nil
}

func (c *LinuxContainer) rsync(src, dst string) error {
	wshPath := path.Join(c.path, "bin", "wsh")
	sockPath := path.Join(c.path, "run", "wshd.sock")

	rsync := exec.Command(
		"rsync",
		"-e", wshPath+" --socket "+sockPath+" --rsh",
		"-r",
		"-p",
		"--links",
		src,
		dst,
	)

	return c.runner.Run(rsync)
}
