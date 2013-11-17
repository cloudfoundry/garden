package fake_backend

import (
	"github.com/vito/garden/backend"
)

type FakeContainer struct {
	Spec backend.ContainerSpec

	StartError error
	Started    bool

	CopyInError error
	CopiedIn    [][]string

	CopyOutError error
	CopiedOut    [][]string

	SpawnError   error
	SpawnedJobID uint32
	Spawned      []backend.JobSpec

	LinkError       error
	LinkedJobResult backend.JobResult
	Linked          []uint32

	LimitBandwidthError error
	LimitedBandwidth    backend.BandwidthLimits

	LimitMemoryError error
	LimitedMemory    backend.MemoryLimits

	NetInError error
	MappedIn [][]uint32
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

func (c *FakeContainer) LimitBandwidth(limits backend.BandwidthLimits) (backend.BandwidthLimits, error) {
	if c.LimitBandwidthError != nil {
		return backend.BandwidthLimits{}, c.LimitBandwidthError
	}

	c.LimitedBandwidth = limits

	return limits, nil
}

func (c *FakeContainer) LimitDisk(backend.DiskLimits) (backend.DiskLimits, error) {
	return backend.DiskLimits{}, nil
}

func (c *FakeContainer) LimitMemory(limits backend.MemoryLimits) (backend.MemoryLimits, error) {
	if c.LimitMemoryError != nil {
		return backend.MemoryLimits{}, c.LimitMemoryError
	}

	c.LimitedMemory = limits

	return limits, nil
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

func (c *FakeContainer) Link(jobID uint32) (backend.JobResult, error) {
	if c.LinkError != nil {
		return backend.JobResult{}, c.LinkError
	}

	c.Linked = append(c.Linked, jobID)

	return c.LinkedJobResult, nil
}

func (c *FakeContainer) Run(backend.JobSpec) (backend.JobResult, error) {
	return backend.JobResult{}, nil
}

func (c *FakeContainer) NetIn(hostPort uint32, containerPort uint32) (uint32, uint32, error) {
	if c.NetInError != nil {
		return 0, 0, c.NetInError
	}

	c.MappedIn = append(c.MappedIn, []uint32{hostPort, containerPort})

	return hostPort, containerPort, nil
}

func (c *FakeContainer) NetOut(string, uint32) error {
	return nil
}
