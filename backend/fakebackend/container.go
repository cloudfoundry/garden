package fakebackend

import (
	"github.com/vito/garden/backend"
)

type FakeContainer struct {
	Spec backend.ContainerSpec
}

func (c *FakeContainer) Handle() string {
	return c.Spec.Handle
}

func (c *FakeContainer) Destroy() error {
	return nil
}

func (c *FakeContainer) Stop(bool, bool) error {
	return nil
}

func (c *FakeContainer) Info() (backend.ContainerInfo, error) {
	return backend.ContainerInfo{}, nil
}

func (c *FakeContainer) CopyIn(string, string) error {
	return nil
}

func (c *FakeContainer) CopyOut(string, string, string) error {
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

func (c *FakeContainer) Spawn(backend.JobSpec) (uint32, error) {
	return 0, nil
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
