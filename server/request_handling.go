package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"code.google.com/p/gogoprotobuf/proto"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var ErrInvalidContentType = errors.New("content-type must be application/json")

func (s *WardenServer) handlePing(w http.ResponseWriter, r *http.Request) {
	err := s.backend.Ping()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	s.writeResponse(w, &protocol.PingResponse{})
}

func (s *WardenServer) handleCapacity(w http.ResponseWriter, r *http.Request) {
	capacity, err := s.backend.Capacity()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.CapacityResponse{
		MemoryInBytes: proto.Uint64(capacity.MemoryInBytes),
		DiskInBytes:   proto.Uint64(capacity.DiskInBytes),
		MaxContainers: proto.Uint64(capacity.MaxContainers),
	})
}

func (s *WardenServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	var request protocol.CreateRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	bindMounts := []warden.BindMount{}

	for _, bm := range request.GetBindMounts() {
		bindMount := warden.BindMount{
			SrcPath: bm.GetSrcPath(),
			DstPath: bm.GetDstPath(),
			Mode:    warden.BindMountMode(bm.GetMode()),
			Origin:  warden.BindMountOrigin(bm.GetOrigin()),
		}

		bindMounts = append(bindMounts, bindMount)
	}

	properties := map[string]string{}

	for _, prop := range request.GetProperties() {
		properties[prop.GetKey()] = prop.GetValue()
	}

	graceTime := s.containerGraceTime

	if request.GraceTime != nil {
		graceTime = time.Duration(request.GetGraceTime()) * time.Second
	}

	container, err := s.backend.Create(warden.ContainerSpec{
		Handle:     request.GetHandle(),
		GraceTime:  graceTime,
		RootFSPath: request.GetRootfs(),
		Network:    request.GetNetwork(),
		BindMounts: bindMounts,
		Properties: properties,
	})

	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Strap(container)

	s.writeResponse(w, &protocol.CreateResponse{
		Handle: proto.String(container.Handle()),
	})
}

func (s *WardenServer) handleList(w http.ResponseWriter, r *http.Request) {
	properties := warden.Properties{}
	for name, vals := range r.URL.Query() {
		if len(vals) > 0 {
			properties[name] = vals[0]
		}
	}

	containers, err := s.backend.Containers(properties)
	if err != nil {
		s.writeError(w, err)
		return
	}

	handles := []string{}

	for _, container := range containers {
		handles = append(handles, container.Handle())
	}

	s.writeResponse(w, &protocol.ListResponse{Handles: handles})
}

func (s *WardenServer) handleDestroy(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	err := s.backend.Destroy(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Defuse(handle)

	s.writeResponse(w, &protocol.DestroyResponse{})
}

func (s *WardenServer) handleStop(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.StopRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	kill := request.GetKill()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.Stop(kill)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.StopResponse{})
}

func (s *WardenServer) handleStreamIn(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	dstPath := r.URL.Query().Get("destination")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.StreamIn(dstPath, r.Body)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.StreamInResponse{})
}

func (s *WardenServer) handleStreamOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	srcPath := r.URL.Query().Get("source")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	reader, err := container.StreamOut(srcPath)
	if err != nil {
		s.writeError(w, err)
		return
	}

	_, err = io.Copy(w, reader)
	if err != nil {
		s.writeError(w, err)
		return
	}
}

func (s *WardenServer) handleLimitBandwidth(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.LimitBandwidthRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.LimitBandwidth(warden.BandwidthLimits{
		RateInBytesPerSecond:      request.GetRate(),
		BurstRateInBytesPerSecond: request.GetBurst(),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}

	limits, err := container.CurrentBandwidthLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitBandwidthResponse{
		Rate:  proto.Uint64(limits.RateInBytesPerSecond),
		Burst: proto.Uint64(limits.BurstRateInBytesPerSecond),
	})
}

func (s *WardenServer) handleCurrentBandwidthLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	limits, err := container.CurrentBandwidthLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitBandwidthResponse{
		Rate:  proto.Uint64(limits.RateInBytesPerSecond),
		Burst: proto.Uint64(limits.BurstRateInBytesPerSecond),
	})
}

func (s *WardenServer) handleLimitMemory(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.LimitMemoryRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	limitInBytes := request.GetLimitInBytes()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInBytes != nil {
		err = container.LimitMemory(warden.MemoryLimits{
			LimitInBytes: limitInBytes,
		})

		if err != nil {
			s.writeError(w, err)
			return
		}
	}

	limits, err := container.CurrentMemoryLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitMemoryResponse{
		LimitInBytes: proto.Uint64(limits.LimitInBytes),
	})
}

func (s *WardenServer) handleCurrentMemoryLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	limits, err := container.CurrentMemoryLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitMemoryResponse{
		LimitInBytes: proto.Uint64(limits.LimitInBytes),
	})
}

func (s *WardenServer) handleLimitDisk(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.LimitDiskRequest
	if !s.readRequest(&request, w, r) {
		return
	}

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

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
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
			s.writeError(w, err)
			return
		}
	}

	limits, err := container.CurrentDiskLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitDiskResponse{
		BlockSoft: proto.Uint64(limits.BlockSoft),
		BlockHard: proto.Uint64(limits.BlockHard),
		InodeSoft: proto.Uint64(limits.InodeSoft),
		InodeHard: proto.Uint64(limits.InodeHard),
		ByteSoft:  proto.Uint64(limits.ByteSoft),
		ByteHard:  proto.Uint64(limits.ByteHard),
	})
}

func (s *WardenServer) handleCurrentDiskLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	limits, err := container.CurrentDiskLimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitDiskResponse{
		BlockSoft: proto.Uint64(limits.BlockSoft),
		BlockHard: proto.Uint64(limits.BlockHard),
		InodeSoft: proto.Uint64(limits.InodeSoft),
		InodeHard: proto.Uint64(limits.InodeHard),
		ByteSoft:  proto.Uint64(limits.ByteSoft),
		ByteHard:  proto.Uint64(limits.ByteHard),
	})
}

func (s *WardenServer) handleLimitCPU(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.LimitCpuRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	limitInShares := request.GetLimitInShares()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInShares != nil {
		err = container.LimitCPU(warden.CPULimits{
			LimitInShares: limitInShares,
		})
		if err != nil {
			s.writeError(w, err)
			return
		}
	}

	limits, err := container.CurrentCPULimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitCpuResponse{
		LimitInShares: proto.Uint64(limits.LimitInShares),
	})
}

func (s *WardenServer) handleCurrentCPULimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	limits, err := container.CurrentCPULimits()
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.LimitCpuResponse{
		LimitInShares: proto.Uint64(limits.LimitInShares),
	})
}

func (s *WardenServer) handleNetIn(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.NetInRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	hostPort := request.GetHostPort()
	containerPort := request.GetContainerPort()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hostPort, containerPort, err = container.NetIn(hostPort, containerPort)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.NetInResponse{
		HostPort:      proto.Uint32(hostPort),
		ContainerPort: proto.Uint32(containerPort),
	})
}

func (s *WardenServer) handleNetOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.NetOutRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	network := request.GetNetwork()
	port := request.GetPort()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	err = container.NetOut(network, port)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.writeResponse(w, &protocol.NetOutResponse{})
}

func (s *WardenServer) handleRun(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.RunRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	path := request.GetPath()
	args := request.GetArgs()
	dir := request.GetDir()
	privileged := request.GetPrivileged()
	env := request.GetEnv()
	tty := request.GetTty()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	processSpec := warden.ProcessSpec{
		Path:       path,
		Args:       args,
		Dir:        dir,
		Privileged: privileged,
		Env:        convertEnv(env),
		TTY:        ttySpecFrom(tty),
	}

	if request.Rlimits != nil {
		processSpec.Limits = resourceLimits(request.Rlimits)
	}

	stdout := make(chan []byte, 1000)
	stderr := make(chan []byte, 1000)

	stdinR, stdinW := io.Pipe()

	processIO := warden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	process, err := container.Run(processSpec, processIO)
	if err != nil {
		s.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err)
		return
	}

	defer conn.Close()

	transport.WriteMessage(conn, &protocol.ProcessPayload{
		ProcessId: proto.Uint32(process.ID()),
	})

	go s.streamInput(json.NewDecoder(br), stdinW, process)

	s.streamProcess(conn, process, stdout, stderr)
}

func (s *WardenServer) streamInput(decoder *json.Decoder, in io.WriteCloser, process warden.Process) {
	for {
		var payload protocol.ProcessPayload
		err := decoder.Decode(&payload)
		if err != nil {
			return
		}

		switch {
		case payload.Tty != nil:
			process.SetTTY(*ttySpecFrom(payload.GetTty()))

		case payload.Source != nil:
			if payload.Data == nil {
				err := in.Close()
				if err != nil {
					return
				}
			} else {
				_, err := in.Write([]byte(payload.GetData()))
				if err != nil {
					return
				}
			}
		default:
			log.Println("received unknown process payload:", payload)
			return
		}
	}
}

func (s *WardenServer) handleAttach(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var processID uint32

	_, err := fmt.Sscanf(r.FormValue(":pid"), "%d", &processID)
	if err != nil {
		s.writeError(w, err)
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	stdout := make(chan []byte, 1000)
	stderr := make(chan []byte, 1000)

	stdinR, stdinW := io.Pipe()

	processIO := warden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	process, err := container.Attach(processID, processIO)
	if err != nil {
		s.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err)
		return
	}

	defer conn.Close()

	go s.streamInput(json.NewDecoder(br), stdinW, process)

	s.streamProcess(conn, process, stdout, stderr)
}

func (s *WardenServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	info, err := container.Info()
	if err != nil {
		s.writeError(w, err)
		return
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

	mappedPorts := []*protocol.InfoResponse_PortMapping{}
	for _, mapping := range info.MappedPorts {
		mappedPorts = append(mappedPorts, &protocol.InfoResponse_PortMapping{
			HostPort:      proto.Uint32(mapping.HostPort),
			ContainerPort: proto.Uint32(mapping.ContainerPort),
		})
	}

	s.writeResponse(w, &protocol.InfoResponse{
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

		MappedPorts: mappedPorts,
	})
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

func (s *WardenServer) writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

func (s *WardenServer) writeResponse(w http.ResponseWriter, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	transport.WriteMessage(w, msg)
}

func (s *WardenServer) readRequest(msg proto.Message, w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		s.writeError(w, ErrInvalidContentType)
		return false
	}

	err := json.NewDecoder(r.Body).Decode(msg)
	if err != nil {
		s.writeError(w, err)
		return false
	}

	return true
}

func convertEnv(env []*protocol.EnvironmentVariable) []string {
	converted := []string{}

	for _, e := range env {
		converted = append(converted, e.GetKey()+"="+e.GetValue())
	}

	return converted
}

func (s *WardenServer) streamProcess(conn net.Conn, process warden.Process, stdout <-chan []byte, stderr <-chan []byte) {
	stdoutSource := protocol.ProcessPayload_stdout
	stderrSource := protocol.ProcessPayload_stderr

	statusCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		status, err := process.Wait()
		if err != nil {
			errCh <- err
		} else {
			statusCh <- status
		}
	}()

	for {
		select {
		case data := <-stdout:
			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId: proto.Uint32(process.ID()),
				Source:    &stdoutSource,
				Data:      proto.String(string(data)),
			})

		case data := <-stderr:
			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId: proto.Uint32(process.ID()),
				Source:    &stderrSource,
				Data:      proto.String(string(data)),
			})

		case status := <-statusCh:
			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId:  proto.Uint32(process.ID()),
				ExitStatus: proto.Uint32(uint32(status)),
			})

			return

		case err := <-errCh:
			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId: proto.Uint32(process.ID()),
				Error:     proto.String(err.Error()),
			})

			return

		case <-s.stopping:
			return
		}
	}
}

func ttySpecFrom(tty *protocol.TTY) *warden.TTYSpec {
	var ttySpec *warden.TTYSpec
	if tty != nil {
		ttySpec = &warden.TTYSpec{}

		windowSize := tty.GetWindowSize()
		if windowSize != nil {
			ttySpec.WindowSize = &warden.WindowSize{
				Columns: int(windowSize.GetColumns()),
				Rows:    int(windowSize.GetRows()),
			}
		}
	}

	return ttySpec
}
