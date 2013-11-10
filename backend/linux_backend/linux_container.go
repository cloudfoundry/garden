package linux_backend

import (
	"github.com/vito/garden/backend"
)

type LinuxContainer struct {
	id   string
	spec backend.ContainerSpec
}

func NewLinuxContainer(id string, spec backend.ContainerSpec) *LinuxContainer {
	return &LinuxContainer{
		id:   id,
		spec: spec,
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
