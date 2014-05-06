package server

import (
	"bufio"
	"io"
	"net"
	"time"

	"github.com/cloudfoundry-incubator/garden/transport"

	"code.google.com/p/gogoprotobuf/proto"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
)

func (s *WardenServer) handlePing(ping *protocol.PingRequest) (proto.Message, error) {
	return &protocol.PingResponse{}, nil
}

func (s *WardenServer) handleEcho(echo *protocol.EchoRequest) (proto.Message, error) {
	return &protocol.EchoResponse{Message: echo.Message}, nil
}

func (s *WardenServer) handleCapacity(echo *protocol.CapacityRequest) (proto.Message, error) {
	capacity, err := s.backend.Capacity()
	if err != nil {
		return nil, err
	}

	return &protocol.CapacityResponse{
		MemoryInBytes: proto.Uint64(capacity.MemoryInBytes),
		DiskInBytes:   proto.Uint64(capacity.DiskInBytes),
	}, nil
}

func (s *WardenServer) handleCreate(create *protocol.CreateRequest) (proto.Message, error) {
	bindMounts := []warden.BindMount{}

	for _, bm := range create.GetBindMounts() {
		bindMount := warden.BindMount{
			SrcPath: bm.GetSrcPath(),
			DstPath: bm.GetDstPath(),
			Mode:    warden.BindMountMode(bm.GetMode()),
			Origin:  warden.BindMountOrigin(bm.GetOrigin()),
		}

		bindMounts = append(bindMounts, bindMount)
	}

	properties := map[string]string{}

	for _, prop := range create.GetProperties() {
		properties[prop.GetKey()] = prop.GetValue()
	}

	graceTime := s.containerGraceTime

	if create.GraceTime != nil {
		graceTime = time.Duration(create.GetGraceTime()) * time.Second
	}

	container, err := s.backend.Create(warden.ContainerSpec{
		Handle:     create.GetHandle(),
		GraceTime:  graceTime,
		RootFSPath: create.GetRootfs(),
		Network:    create.GetNetwork(),
		BindMounts: bindMounts,
		Properties: properties,
	})

	if err != nil {
		return nil, err
	}

	s.bomberman.Strap(container)

	return &protocol.CreateResponse{
		Handle: proto.String(container.Handle()),
	}, nil
}

func (s *WardenServer) handleDestroy(destroy *protocol.DestroyRequest) (proto.Message, error) {
	handle := destroy.GetHandle()

	err := s.backend.Destroy(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Defuse(handle)

	return &protocol.DestroyResponse{}, nil
}

func (s *WardenServer) handleList(list *protocol.ListRequest) (proto.Message, error) {
	properties := warden.Properties{}
	for _, prop := range list.GetProperties() {
		properties[prop.GetKey()] = prop.GetValue()
	}

	containers, err := s.backend.Containers(properties)
	if err != nil {
		return nil, err
	}

	handles := []string{}

	for _, container := range containers {
		handles = append(handles, container.Handle())
	}

	return &protocol.ListResponse{Handles: handles}, nil
}

func (s *WardenServer) handleStop(request *protocol.StopRequest) (proto.Message, error) {
	handle := request.GetHandle()
	kill := request.GetKill()
	background := request.GetBackground()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if background {
		go container.Stop(kill)
	} else {
		err = container.Stop(kill)
		if err != nil {
			return nil, err
		}
	}

	return &protocol.StopResponse{}, nil
}

func (s *WardenServer) handleStreamIn(conn net.Conn, reader *bufio.Reader, request *protocol.StreamInRequest) (proto.Message, error) {
	handle := request.GetHandle()
	dstPath := request.GetDstPath()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	streamWriter, err := container.StreamIn(dstPath)
	if err != nil {
		return nil, err
	}

	_, err = protocol.Messages(&protocol.StreamInResponse{}).WriteTo(conn)
	if err != nil {
		return nil, err
	}

	streamReader := transport.NewProtobufStreamReader(reader)

	_, err = io.Copy(streamWriter, streamReader)
	if err != nil {
		return nil, err
	}

	return nil, streamWriter.Close()
}

func (s *WardenServer) handleStreamOut(conn net.Conn, request *protocol.StreamOutRequest) (proto.Message, error) {
	handle := request.GetHandle()
	srcPath := request.GetSrcPath()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	_, err = protocol.Messages(&protocol.StreamOutResponse{}).WriteTo(conn)
	if err != nil {
		return nil, err
	}

	writer := transport.NewProtobufStreamWriter(conn)

	reader, err := container.StreamOut(srcPath)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(writer, reader)
	if err != nil {
		return nil, err
	}

	return nil, writer.Close()
}

func (s *WardenServer) handleLimitBandwidth(request *protocol.LimitBandwidthRequest) (proto.Message, error) {
	handle := request.GetHandle()
	rate := request.GetRate()
	burst := request.GetBurst()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.LimitBandwidth(warden.BandwidthLimits{
		RateInBytesPerSecond:      rate,
		BurstRateInBytesPerSecond: burst,
	})
	if err != nil {
		return nil, err
	}

	limits, err := container.CurrentBandwidthLimits()
	if err != nil {
		return nil, err
	}

	return &protocol.LimitBandwidthResponse{
		Rate:  proto.Uint64(limits.RateInBytesPerSecond),
		Burst: proto.Uint64(limits.BurstRateInBytesPerSecond),
	}, nil
}

func (s *WardenServer) handleLimitMemory(request *protocol.LimitMemoryRequest) (proto.Message, error) {
	handle := request.GetHandle()
	limitInBytes := request.GetLimitInBytes()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInBytes != nil {
		err = container.LimitMemory(warden.MemoryLimits{
			LimitInBytes: limitInBytes,
		})

		if err != nil {
			return nil, err
		}
	}

	limits, err := container.CurrentMemoryLimits()
	if err != nil {
		return nil, err
	}

	return &protocol.LimitMemoryResponse{
		LimitInBytes: proto.Uint64(limits.LimitInBytes),
	}, nil
}

func (s *WardenServer) handleLimitDisk(request *protocol.LimitDiskRequest) (proto.Message, error) {
	handle := request.GetHandle()
	blockSoft := request.GetBlockSoft()
	blockHard := request.GetBlockHard()
	inodeSoft := request.GetInodeSoft()
	inodeHard := request.GetInodeHard()
	byteSoft := request.GetByteSoft()
	byteHard := request.GetByteHard()

	settingLimit := false

	if request.BlockSoft != nil || request.BlockHard != nil ||
		request.InodeSoft != nil || request.InodeHard != nil ||
		request.ByteSoft != nil || request.ByteHard != nil {
		settingLimit = true
	}

	if request.Block != nil {
		blockHard = request.GetBlock()
		settingLimit = true
	}

	if request.BlockLimit != nil {
		blockHard = request.GetBlockLimit()
		settingLimit = true
	}

	if request.Inode != nil {
		inodeHard = request.GetInode()
		settingLimit = true
	}

	if request.InodeLimit != nil {
		inodeHard = request.GetInodeLimit()
		settingLimit = true
	}

	if request.Byte != nil {
		byteHard = request.GetByte()
		settingLimit = true
	}

	if request.ByteLimit != nil {
		byteHard = request.GetByteLimit()
		settingLimit = true
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if settingLimit {
		err = container.LimitDisk(warden.DiskLimits{
			BlockSoft: blockSoft,
			BlockHard: blockHard,
			InodeSoft: inodeSoft,
			InodeHard: inodeHard,
			ByteSoft:  byteSoft,
			ByteHard:  byteHard,
		})
		if err != nil {
			return nil, err
		}
	}

	limits, err := container.CurrentDiskLimits()
	if err != nil {
		return nil, err
	}

	return &protocol.LimitDiskResponse{
		BlockSoft: proto.Uint64(limits.BlockSoft),
		BlockHard: proto.Uint64(limits.BlockHard),
		InodeSoft: proto.Uint64(limits.InodeSoft),
		InodeHard: proto.Uint64(limits.InodeHard),
		ByteSoft:  proto.Uint64(limits.ByteSoft),
		ByteHard:  proto.Uint64(limits.ByteHard),
	}, nil
}

func (s *WardenServer) handleLimitCpu(request *protocol.LimitCpuRequest) (proto.Message, error) {
	handle := request.GetHandle()
	limitInShares := request.GetLimitInShares()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInShares != nil {
		err = container.LimitCPU(warden.CPULimits{
			LimitInShares: limitInShares,
		})
		if err != nil {
			return nil, err
		}
	}

	limits, err := container.CurrentCPULimits()
	if err != nil {
		return nil, err
	}

	return &protocol.LimitCpuResponse{
		LimitInShares: proto.Uint64(limits.LimitInShares),
	}, nil
}

func (s *WardenServer) handleNetIn(request *protocol.NetInRequest) (proto.Message, error) {
	handle := request.GetHandle()
	hostPort := request.GetHostPort()
	containerPort := request.GetContainerPort()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hostPort, containerPort, err = container.NetIn(hostPort, containerPort)
	if err != nil {
		return nil, err
	}

	return &protocol.NetInResponse{
		HostPort:      proto.Uint32(hostPort),
		ContainerPort: proto.Uint32(containerPort),
	}, nil
}

func (s *WardenServer) handleNetOut(request *protocol.NetOutRequest) (proto.Message, error) {
	handle := request.GetHandle()
	network := request.GetNetwork()
	port := request.GetPort()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.NetOut(network, port)
	if err != nil {
		return nil, err
	}

	return &protocol.NetOutResponse{}, nil
}

func (s *WardenServer) streamProcessToConnection(processID uint32, stream <-chan warden.ProcessStream, conn net.Conn) proto.Message {
	for payload := range stream {
		if payload.ExitStatus != nil {
			return &protocol.ProcessPayload{
				ProcessId:  proto.Uint32(processID),
				ExitStatus: proto.Uint32(*payload.ExitStatus),
			}
		}

		var payloadSource protocol.ProcessPayload_Source

		switch payload.Source {
		case warden.ProcessStreamSourceStdout:
			payloadSource = protocol.ProcessPayload_stdout
		case warden.ProcessStreamSourceStderr:
			payloadSource = protocol.ProcessPayload_stderr
		case warden.ProcessStreamSourceStdin:
			payloadSource = protocol.ProcessPayload_stdin
		}

		protocol.Messages(&protocol.ProcessPayload{
			ProcessId: proto.Uint32(processID),
			Source:    &payloadSource,
			Data:      proto.String(string(payload.Data)),
		}).WriteTo(conn)
	}

	return nil
}

func convertEnvironmentVariables(environmentVariables []*protocol.EnvironmentVariable) []warden.EnvironmentVariable {
	convertedEnvironmentVariables := []warden.EnvironmentVariable{}

	for _, env := range environmentVariables {
		convertedEnvironmentVariable := warden.EnvironmentVariable{
			Key:   env.GetKey(),
			Value: env.GetValue(),
		}
		convertedEnvironmentVariables = append(convertedEnvironmentVariables, convertedEnvironmentVariable)
	}

	return convertedEnvironmentVariables
}

func (s *WardenServer) handleRun(conn net.Conn, request *protocol.RunRequest) (proto.Message, error) {
	handle := request.GetHandle()
	script := request.GetScript()
	privileged := request.GetPrivileged()
	env := request.GetEnv()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	ProcessSpec := warden.ProcessSpec{
		Script:               script,
		Privileged:           privileged,
		EnvironmentVariables: convertEnvironmentVariables(env),
	}

	if request.Rlimits != nil {
		ProcessSpec.Limits = resourceLimits(request.Rlimits)
	}

	processID, stream, err := container.Run(ProcessSpec)
	if err != nil {
		return nil, err
	}

	protocol.Messages(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(processID),
	}).WriteTo(conn)

	return s.streamProcessToConnection(processID, stream, conn), nil
}

func (s *WardenServer) handleAttach(conn net.Conn, request *protocol.AttachRequest) (proto.Message, error) {
	handle := request.GetHandle()
	processID := request.GetProcessId()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	stream, err := container.Attach(processID)
	if err != nil {
		return nil, err
	}

	return s.streamProcessToConnection(processID, stream, conn), nil
}

func (s *WardenServer) handleInfo(request *protocol.InfoRequest) (proto.Message, error) {
	handle := request.GetHandle()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		return nil, err
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	info, err := container.Info()
	if err != nil {
		return nil, err
	}

	properties := []*protocol.Property{}
	for key, val := range info.Properties {
		properties = append(properties, &protocol.Property{
			Key:   proto.String(key),
			Value: proto.String(val),
		})
	}

	processIDs := make([]uint64, len(info.ProcessIDs))
	for i, processID := range info.ProcessIDs {
		processIDs[i] = uint64(processID)
	}

	return &protocol.InfoResponse{
		State:         proto.String(info.State),
		Events:        info.Events,
		HostIp:        proto.String(info.HostIP),
		ContainerIp:   proto.String(info.ContainerIP),
		ContainerPath: proto.String(info.ContainerPath),
		ProcessIds:    processIDs,

		Properties: properties,

		MemoryStat: &protocol.InfoResponse_MemoryStat{
			Cache:                   proto.Uint64(info.MemoryStat.Cache),
			Rss:                     proto.Uint64(info.MemoryStat.Rss),
			MappedFile:              proto.Uint64(info.MemoryStat.MappedFile),
			Pgpgin:                  proto.Uint64(info.MemoryStat.Pgpgin),
			Pgpgout:                 proto.Uint64(info.MemoryStat.Pgpgout),
			Swap:                    proto.Uint64(info.MemoryStat.Swap),
			Pgfault:                 proto.Uint64(info.MemoryStat.Pgfault),
			Pgmajfault:              proto.Uint64(info.MemoryStat.Pgmajfault),
			InactiveAnon:            proto.Uint64(info.MemoryStat.InactiveAnon),
			ActiveAnon:              proto.Uint64(info.MemoryStat.ActiveAnon),
			InactiveFile:            proto.Uint64(info.MemoryStat.InactiveFile),
			ActiveFile:              proto.Uint64(info.MemoryStat.ActiveFile),
			Unevictable:             proto.Uint64(info.MemoryStat.Unevictable),
			HierarchicalMemoryLimit: proto.Uint64(info.MemoryStat.HierarchicalMemoryLimit),
			HierarchicalMemswLimit:  proto.Uint64(info.MemoryStat.HierarchicalMemswLimit),
			TotalCache:              proto.Uint64(info.MemoryStat.TotalCache),
			TotalRss:                proto.Uint64(info.MemoryStat.TotalRss),
			TotalMappedFile:         proto.Uint64(info.MemoryStat.TotalMappedFile),
			TotalPgpgin:             proto.Uint64(info.MemoryStat.TotalPgpgin),
			TotalPgpgout:            proto.Uint64(info.MemoryStat.TotalPgpgout),
			TotalSwap:               proto.Uint64(info.MemoryStat.TotalSwap),
			TotalPgfault:            proto.Uint64(info.MemoryStat.TotalPgfault),
			TotalPgmajfault:         proto.Uint64(info.MemoryStat.TotalPgmajfault),
			TotalInactiveAnon:       proto.Uint64(info.MemoryStat.TotalInactiveAnon),
			TotalActiveAnon:         proto.Uint64(info.MemoryStat.TotalActiveAnon),
			TotalInactiveFile:       proto.Uint64(info.MemoryStat.TotalInactiveFile),
			TotalActiveFile:         proto.Uint64(info.MemoryStat.TotalActiveFile),
			TotalUnevictable:        proto.Uint64(info.MemoryStat.TotalUnevictable),
		},

		CpuStat: &protocol.InfoResponse_CpuStat{
			Usage:  proto.Uint64(info.CPUStat.Usage),
			User:   proto.Uint64(info.CPUStat.User),
			System: proto.Uint64(info.CPUStat.System),
		},

		DiskStat: &protocol.InfoResponse_DiskStat{
			BytesUsed:  proto.Uint64(info.DiskStat.BytesUsed),
			InodesUsed: proto.Uint64(info.DiskStat.InodesUsed),
		},

		BandwidthStat: &protocol.InfoResponse_BandwidthStat{
			InRate:   proto.Uint64(info.BandwidthStat.InRate),
			InBurst:  proto.Uint64(info.BandwidthStat.InBurst),
			OutRate:  proto.Uint64(info.BandwidthStat.OutRate),
			OutBurst: proto.Uint64(info.BandwidthStat.OutBurst),
		},
	}, nil
}

func resourceLimits(limits *protocol.ResourceLimits) warden.ResourceLimits {
	return warden.ResourceLimits{
		As:         limits.As,
		Core:       limits.Core,
		Cpu:        limits.Cpu,
		Data:       limits.Data,
		Fsize:      limits.Fsize,
		Locks:      limits.Locks,
		Memlock:    limits.Memlock,
		Msgqueue:   limits.Msgqueue,
		Nice:       limits.Nice,
		Nofile:     limits.Nofile,
		Nproc:      limits.Nproc,
		Rss:        limits.Rss,
		Rtprio:     limits.Rtprio,
		Sigpending: limits.Sigpending,
		Stack:      limits.Stack,
	}
}
