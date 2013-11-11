package fake_backend

import (
	"github.com/vito/garden/backend"
)

type FakeContainer struct {
	Spec backend.ContainerSpec

	Started bool

	StartError error

	CopyInError  error
	CopyOutError error

	SpawnedJobID uint32
	SpawnError   error

	CopiedIn  [][]string
	CopiedOut [][]string
	Spawned   []backend.JobSpec
}

func NewFakeContainer(spec backend.ContainerSpec) *FakeContainer {
	return &FakeContainer{Spec: spec}
}

func (c *FakeContainer) ID() string {
	return c.Spec.Handle
}

func (c *FakeContainer) Handle() string {
	return c.Spec.Handle
}

func (c *FakeContainer) Start() error {
	if c.StartError != nil {
		return c.StartError
	}

	c.Started = true

	return nil
}

func (c *FakeContainer) Stop(bool) error {
	return nil
}

func (c *FakeContainer) Info() (backend.ContainerInfo, error) {
	return backend.ContainerInfo{}, nil
}

func (c *FakeContainer) CopyIn(src, dst string) error {
	if c.CopyInError != nil {
		return c.CopyInError
	}

	c.CopiedIn = append(c.CopiedIn, []string{src, dst})

	return nil
}

func (c *FakeContainer) CopyOut(src, dst, owner string) error {
	if c.CopyOutError != nil {
		return c.CopyOutError
	}

	c.CopiedOut = append(c.CopiedOut, []string{src, dst, owner})

	return nil
}

func (c *FakeContainer) LimitBandwidth(backend.BandwidthLimits) (backend.BandwidthLimits, error) {
	return backend.BandwidthLimits{}, nil
}

func (c *FakeContainer) LimitDisk(backend.DiskLimits) (backend.DiskLimits, error) {
	return backend.DiskLimits{}, nil
}

func (c *FakeContainer) LimitMemory(backend.MemoryLimits) (backend.MemoryLimits, error) {
	return backend.MemoryLimits{}, nil
}

func (c *FakeContainer) Spawn(spec backend.JobSpec) (uint32, error) {
	if c.SpawnError != nil {
		return 0, c.SpawnError
	}

	c.Spawned = append(c.Spawned, spec)

	return c.SpawnedJobID, nil
}

func (c *FakeContainer) Stream(uint32) (<-chan backend.JobStream, error) {
	return nil, nil
}

func (c *FakeContainer) Link(uint32) (backend.JobResult, error) {
	return backend.JobResult{}, nil
}

func (c *FakeContainer) Run(backend.JobSpec) (backend.JobResult, error) {
	return backend.JobResult{}, nil
}

func (c *FakeContainer) NetIn(uint32, uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (c *FakeContainer) NetOut(string, uint32) error {
	return nil
}
