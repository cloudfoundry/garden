package client

import (
	"io"

	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type container struct {
	handle string

	connection connection.Connection
}

func newContainer(handle string, connection connection.Connection) warden.Container {
	return &container{
		handle: handle,

		connection: connection,
	}
}

func (container *container) Handle() string {
	return container.handle
}

func (container *container) Stop(kill bool) error {
	return container.connection.Stop(container.handle, kill)
}

func (container *container) Info() (warden.ContainerInfo, error) {
	return container.connection.Info(container.handle)
}

func (container *container) StreamIn(dstPath string, reader io.Reader) error {
	return container.connection.StreamIn(container.handle, dstPath, reader)
}

func (container *container) StreamOut(srcPath string) (io.ReadCloser, error) {
	return container.connection.StreamOut(container.handle, srcPath)
}

func (container *container) LimitBandwidth(limits warden.BandwidthLimits) error {
	_, err := container.connection.LimitBandwidth(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentBandwidthLimits() (warden.BandwidthLimits, error) {
	return container.connection.CurrentBandwidthLimits(container.handle)
}

func (container *container) LimitCPU(limits warden.CPULimits) error {
	_, err := container.connection.LimitCPU(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentCPULimits() (warden.CPULimits, error) {
	return container.connection.CurrentCPULimits(container.handle)
}

func (container *container) LimitDisk(limits warden.DiskLimits) error {
	_, err := container.connection.LimitDisk(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentDiskLimits() (warden.DiskLimits, error) {
	return container.connection.CurrentDiskLimits(container.handle)
}

func (container *container) LimitMemory(limits warden.MemoryLimits) error {
	_, err := container.connection.LimitMemory(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentMemoryLimits() (warden.MemoryLimits, error) {
	return container.connection.CurrentMemoryLimits(container.handle)
}

func (container *container) Run(spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
	return container.connection.Run(container.handle, spec, io)
}

func (container *container) Attach(processID uint32, io warden.ProcessIO) (warden.Process, error) {
	return container.connection.Attach(container.handle, processID, io)
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return container.connection.NetIn(container.handle, hostPort, containerPort)
}

func (container *container) NetOut(network string, port uint32) error {
	return container.connection.NetOut(container.handle, network, port)
}
