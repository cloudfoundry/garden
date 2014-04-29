package client

import (
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type container struct {
	handle string

	pool *connectionPool
}

func newContainer(handle string, pool *connectionPool) *container {
	return &container{
		handle: handle,

		pool: pool,
	}
}

func (container *container) Handle() string {
	return container.handle
}

func (container *container) Stop(kill bool) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.Stop(container.handle, false, kill)
}

func (container *container) Info() (warden.ContainerInfo, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.Info(container.handle)
}

func (container *container) CopyIn(srcPath, dstPath string) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.CopyIn(container.handle, srcPath, dstPath)
}

func (container *container) CopyOut(srcPath, dstPath, owner string) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.CopyOut(container.handle, srcPath, dstPath, owner)
}

func (container *container) LimitBandwidth(limits warden.BandwidthLimits) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.LimitBandwidth(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentBandwidthLimits() (warden.BandwidthLimits, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.LimitBandwidth(container.handle, warden.BandwidthLimits{})
}

func (container *container) LimitCPU(limits warden.CPULimits) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.LimitCPU(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentCPULimits() (warden.CPULimits, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.LimitCPU(container.handle, warden.CPULimits{})
}

func (container *container) LimitDisk(limits warden.DiskLimits) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.LimitDisk(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentDiskLimits() (warden.DiskLimits, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.LimitDisk(container.handle, warden.DiskLimits{})
}

func (container *container) LimitMemory(limits warden.MemoryLimits) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.LimitMemory(container.handle, limits)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CurrentMemoryLimits() (warden.MemoryLimits, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.LimitMemory(container.handle, warden.MemoryLimits{})
}

func (container *container) Run(spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
	conn := container.pool.Acquire()

	pid, stream, err := conn.Run(container.handle, spec)
	if err != nil {
		container.pool.Release(conn)
		return 0, nil, err
	}

	outStream := make(chan warden.ProcessStream)

	go container.streamPayloads(outStream, stream, conn)

	return pid, outStream, nil
}

func (container *container) Attach(processID uint32) (<-chan warden.ProcessStream, error) {
	conn := container.pool.Acquire()

	stream, err := conn.Attach(container.handle, processID)
	if err != nil {
		container.pool.Release(conn)
		return nil, err
	}

	outStream := make(chan warden.ProcessStream)

	go container.streamPayloads(outStream, stream, conn)

	return outStream, nil
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.NetIn(container.handle, hostPort, containerPort)
}

func (container *container) NetOut(network string, port uint32) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	return conn.NetOut(container.handle, network, port)
}

func (container *container) streamPayloads(out chan<- warden.ProcessStream, in <-chan warden.ProcessStream, conn connection.Connection) {
	defer container.pool.Release(conn)

	for chunk := range in {
		out <- chunk
	}

	close(out)
}
