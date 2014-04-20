package client

import (
	"github.com/cloudfoundry-incubator/garden/client/connection"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
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

	_, err := conn.Stop(container.handle, false, kill)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) Info() (warden.ContainerInfo, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	info, err := conn.Info(container.handle)
	if err != nil {
		return warden.ContainerInfo{}, err
	}

	processIDs := []uint32{}
	for _, pid := range info.GetProcessIds() {
		processIDs = append(processIDs, uint32(pid))
	}

	bandwidthStat := info.GetBandwidthStat()
	cpuStat := info.GetCpuStat()
	diskStat := info.GetDiskStat()
	memoryStat := info.GetMemoryStat()

	return warden.ContainerInfo{
		State:  info.GetState(),
		Events: info.GetEvents(),

		HostIP:      info.GetHostIp(),
		ContainerIP: info.GetContainerIp(),

		ContainerPath: info.GetContainerPath(),

		ProcessIDs: processIDs,

		BandwidthStat: warden.ContainerBandwidthStat{
			InRate:   bandwidthStat.GetInRate(),
			InBurst:  bandwidthStat.GetInBurst(),
			OutRate:  bandwidthStat.GetOutRate(),
			OutBurst: bandwidthStat.GetOutBurst(),
		},

		CPUStat: warden.ContainerCPUStat{
			Usage:  cpuStat.GetUsage(),
			User:   cpuStat.GetUser(),
			System: cpuStat.GetSystem(),
		},

		DiskStat: warden.ContainerDiskStat{
			BytesUsed:  diskStat.GetBytesUsed(),
			InodesUsed: diskStat.GetInodesUsed(),
		},

		MemoryStat: warden.ContainerMemoryStat{
			Cache:                   memoryStat.GetCache(),
			Rss:                     memoryStat.GetRss(),
			MappedFile:              memoryStat.GetMappedFile(),
			Pgpgin:                  memoryStat.GetPgpgin(),
			Pgpgout:                 memoryStat.GetPgpgout(),
			Swap:                    memoryStat.GetSwap(),
			Pgfault:                 memoryStat.GetPgfault(),
			Pgmajfault:              memoryStat.GetPgmajfault(),
			InactiveAnon:            memoryStat.GetInactiveAnon(),
			ActiveAnon:              memoryStat.GetActiveAnon(),
			InactiveFile:            memoryStat.GetInactiveFile(),
			ActiveFile:              memoryStat.GetActiveFile(),
			Unevictable:             memoryStat.GetUnevictable(),
			HierarchicalMemoryLimit: memoryStat.GetHierarchicalMemoryLimit(),
			HierarchicalMemswLimit:  memoryStat.GetHierarchicalMemswLimit(),
			TotalCache:              memoryStat.GetTotalCache(),
			TotalRss:                memoryStat.GetTotalRss(),
			TotalMappedFile:         memoryStat.GetTotalMappedFile(),
			TotalPgpgin:             memoryStat.GetTotalPgpgin(),
			TotalPgpgout:            memoryStat.GetTotalPgpgout(),
			TotalSwap:               memoryStat.GetTotalSwap(),
			TotalPgfault:            memoryStat.GetTotalPgfault(),
			TotalPgmajfault:         memoryStat.GetTotalPgmajfault(),
			TotalInactiveAnon:       memoryStat.GetTotalInactiveAnon(),
			TotalActiveAnon:         memoryStat.GetTotalActiveAnon(),
			TotalInactiveFile:       memoryStat.GetTotalInactiveFile(),
			TotalActiveFile:         memoryStat.GetTotalActiveFile(),
			TotalUnevictable:        memoryStat.GetTotalUnevictable(),
		},
	}, nil
}

func (container *container) CopyIn(srcPath, dstPath string) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.CopyIn(container.handle, srcPath, dstPath)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) CopyOut(srcPath, dstPath, owner string) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.CopyOut(container.handle, srcPath, dstPath, owner)
	if err != nil {
		return err
	}

	return nil
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

	res, err := conn.LimitBandwidth(container.handle, warden.BandwidthLimits{})
	if err != nil {
		return warden.BandwidthLimits{}, err
	}

	return warden.BandwidthLimits{
		RateInBytesPerSecond:      res.GetRate(),
		BurstRateInBytesPerSecond: res.GetBurst(),
	}, nil
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

	res, err := conn.LimitCPU(container.handle, warden.CPULimits{})
	if err != nil {
		return warden.CPULimits{}, err
	}

	return warden.CPULimits{
		LimitInShares: res.GetLimitInShares(),
	}, nil
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

	res, err := conn.LimitDisk(container.handle, warden.DiskLimits{})
	if err != nil {
		return warden.DiskLimits{}, err
	}

	return warden.DiskLimits{
		BlockLimit: res.GetBlockLimit(),
		Block:      res.GetBlock(),
		BlockSoft:  res.GetBlockSoft(),
		BlockHard:  res.GetBlockHard(),
		InodeLimit: res.GetInodeLimit(),
		Inode:      res.GetInode(),
		InodeSoft:  res.GetInodeSoft(),
		InodeHard:  res.GetInodeHard(),
		ByteLimit:  res.GetByteLimit(),
		Byte:       res.GetByte(),
		ByteSoft:   res.GetByteSoft(),
		ByteHard:   res.GetByteHard(),
	}, nil
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

	res, err := conn.LimitMemory(container.handle, warden.MemoryLimits{})
	if err != nil {
		return warden.MemoryLimits{}, err
	}

	return warden.MemoryLimits{
		LimitInBytes: res.GetLimitInBytes(),
	}, nil
}

func (container *container) Run(spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
	conn := container.pool.Acquire()

	res, payloads, err := conn.Run(container.handle, spec)
	if err != nil {
		container.pool.Release(conn)
		return 0, nil, err
	}

	stream := make(chan warden.ProcessStream)

	go container.streamPayloads(stream, payloads, conn)

	return res.GetProcessId(), stream, nil
}

func (container *container) Attach(processID uint32) (<-chan warden.ProcessStream, error) {
	conn := container.pool.Acquire()

	payloads, err := conn.Attach(container.handle, processID)
	if err != nil {
		container.pool.Release(conn)
		return nil, err
	}

	stream := make(chan warden.ProcessStream)

	go container.streamPayloads(stream, payloads, conn)

	return stream, nil
}

func (container *container) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	res, err := conn.NetIn(container.handle, hostPort, containerPort)
	if err != nil {
		return 0, 0, err
	}

	return res.GetHostPort(), res.GetContainerPort(), nil
}

func (container *container) NetOut(network string, port uint32) error {
	conn := container.pool.Acquire()
	defer container.pool.Release(conn)

	_, err := conn.NetOut(container.handle, network, port)
	if err != nil {
		return err
	}

	return nil
}

func (container *container) streamPayloads(stream chan<- warden.ProcessStream, payloads <-chan *protocol.ProcessPayload, conn connection.Connection) {
	defer container.pool.Release(conn)

	for {
		payload := <-payloads

		if payload.ExitStatus != nil {
			stream <- warden.ProcessStream{
				ExitStatus: payload.ExitStatus,
			}

			close(stream)

			return
		} else {
			var source warden.ProcessStreamSource

			switch payload.GetSource() {
			case protocol.ProcessPayload_stdin:
				source = warden.ProcessStreamSourceStdin
			case protocol.ProcessPayload_stdout:
				source = warden.ProcessStreamSourceStdout
			case protocol.ProcessPayload_stderr:
				source = warden.ProcessStreamSourceStderr
			}

			stream <- warden.ProcessStream{
				Source: source,
				Data:   []byte(payload.GetData()),
			}
		}
	}
}
