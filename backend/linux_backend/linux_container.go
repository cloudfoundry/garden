package linux_backend

import (
	"os/exec"
	"path"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/command_runner"
)

type LinuxContainer struct {
	id   string
	path string

	spec backend.ContainerSpec

	runner command_runner.CommandRunner
}

func NewLinuxContainer(id, path string, spec backend.ContainerSpec, runner command_runner.CommandRunner) *LinuxContainer {
	return &LinuxContainer{
		id:   id,
		path: path,

		spec: spec,

		runner: runner,
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

	err := c.runner.Run(start)
	if err != nil {
		return err
	}

	return nil
}

func (c *LinuxContainer) Stop(bool, bool) error {
	return nil
}

func (c *LinuxContainer) Info() (backend.ContainerInfo, error) {
	return backend.ContainerInfo{}, nil
}

func (c *LinuxContainer) CopyIn(src, dst string) error {
	return nil
}

func (c *LinuxContainer) CopyOut(src, dst, owner string) error {
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
	return 0, nil
}

func (c *LinuxContainer) Stream(uint32) (<-chan backend.JobStream, error) {
	return nil, nil
}

func (c *LinuxContainer) Link(jobID uint32) (backend.JobResult, error) {
	return backend.JobResult{}, nil
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
