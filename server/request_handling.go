package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/cloudfoundry-incubator/garden"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/pivotal-golang/lager"
)

var ErrInvalidContentType = errors.New("content-type must be application/json")
var ErrConcurrentDestroy = errors.New("container already being destroyed")

func (s *GardenServer) handlePing(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("ping")

	err := s.backend.Ping()
	if err != nil {
		hLog.Error("failed", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	s.writeResponse(w, &protocol.PingResponse{})
}

func (s *GardenServer) handleCapacity(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("capacity")

	capacity, err := s.backend.Capacity()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.writeResponse(w, &protocol.CapacityResponse{
		MemoryInBytes: proto.Uint64(capacity.MemoryInBytes),
		DiskInBytes:   proto.Uint64(capacity.DiskInBytes),
		MaxContainers: proto.Uint64(capacity.MaxContainers),
	})
}

func (s *GardenServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	var request protocol.CreateRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	hLog := s.logger.Session("create", lager.Data{
		"request": request,
	})

	bindMounts := []garden.BindMount{}

	for _, bm := range request.GetBindMounts() {
		bindMount := garden.BindMount{
			SrcPath: bm.GetSrcPath(),
			DstPath: bm.GetDstPath(),
			Mode:    garden.BindMountMode(bm.GetMode()),
			Origin:  garden.BindMountOrigin(bm.GetOrigin()),
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

	hLog.Debug("creating")

	container, err := s.backend.Create(garden.ContainerSpec{
		Handle:     request.GetHandle(),
		GraceTime:  graceTime,
		RootFSPath: request.GetRootfs(),
		Network:    request.GetNetwork(),
		BindMounts: bindMounts,
		Properties: properties,
		Env:        convertEnv(request.GetEnv()),
		Privileged: request.GetPrivileged(),
	})
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("created")

	s.bomberman.Strap(container)

	s.writeResponse(w, &protocol.CreateResponse{
		Handle: proto.String(container.Handle()),
	})
}

func (s *GardenServer) handleList(w http.ResponseWriter, r *http.Request) {
	properties := garden.Properties{}
	for name, vals := range r.URL.Query() {
		if len(vals) > 0 {
			properties[name] = vals[0]
		}
	}

	hLog := s.logger.Session("list", lager.Data{
		"properties": properties,
	})

	containers, err := s.backend.Containers(properties)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	handles := []string{}

	for _, container := range containers {
		handles = append(handles, container.Handle())
	}

	s.writeResponse(w, &protocol.ListResponse{Handles: handles})
}

func (s *GardenServer) handleDestroy(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("destroy", lager.Data{
		"handle": handle,
	})

	s.destroysL.Lock()

	_, alreadyDestroying := s.destroys[handle]
	if !alreadyDestroying {
		s.destroys[handle] = struct{}{}
	}

	s.destroysL.Unlock()

	if alreadyDestroying {
		s.writeError(w, ErrConcurrentDestroy, hLog)
		return
	}

	hLog.Debug("destroying")

	err := s.backend.Destroy(handle)

	if !alreadyDestroying {
		s.destroysL.Lock()
		delete(s.destroys, handle)
		s.destroysL.Unlock()
	}

	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("destroyed")

	s.bomberman.Defuse(handle)

	s.writeResponse(w, &protocol.DestroyResponse{})
}

func (s *GardenServer) handleStop(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("stop", lager.Data{
		"handle": handle,
	})

	var request protocol.StopRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	kill := request.GetKill()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("stopping")

	err = container.Stop(kill)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("stopped")

	s.writeResponse(w, &protocol.StopResponse{})
}

func (s *GardenServer) handleStreamIn(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	dstPath := r.URL.Query().Get("destination")

	hLog := s.logger.Session("stream-in", lager.Data{
		"handle":      handle,
		"destination": dstPath,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("streaming-in")

	err = container.StreamIn(dstPath, r.Body)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("streamed-in")

	s.writeResponse(w, &protocol.StreamInResponse{})
}

func (s *GardenServer) handleStreamOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	srcPath := r.URL.Query().Get("source")

	hLog := s.logger.Session("stream-out", lager.Data{
		"handle": handle,
		"source": srcPath,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("streaming-out")

	reader, err := container.StreamOut(srcPath)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	n, err := io.Copy(w, reader)
	if err != nil {
		if err := reader.Close(); err != nil {
			hLog.Error("failed-to-close", err)
		}

		if n == 0 {
			s.writeError(w, err, hLog)
		}

		return
	}

	hLog.Info("streamed-out")
}

func (s *GardenServer) handleLimitBandwidth(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var request protocol.LimitBandwidthRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	hLog := s.logger.Session("limit-bandwidth", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	requestedLimits := garden.BandwidthLimits{
		RateInBytesPerSecond:      request.GetRate(),
		BurstRateInBytesPerSecond: request.GetBurst(),
	}

	hLog.Debug("limiting", lager.Data{
		"requested-limits": requestedLimits,
	})

	err = container.LimitBandwidth(requestedLimits)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	limits, err := container.CurrentBandwidthLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("limited", lager.Data{
		"resulting-limits": limits,
	})

	s.writeResponse(w, &protocol.LimitBandwidthResponse{
		Rate:  proto.Uint64(limits.RateInBytesPerSecond),
		Burst: proto.Uint64(limits.BurstRateInBytesPerSecond),
	})
}

func (s *GardenServer) handleCurrentBandwidthLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("current-bandwidth-limits", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("getting")

	limits, err := container.CurrentBandwidthLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got", lager.Data{
		"limits": limits,
	})

	s.writeResponse(w, &protocol.LimitBandwidthResponse{
		Rate:  proto.Uint64(limits.RateInBytesPerSecond),
		Burst: proto.Uint64(limits.BurstRateInBytesPerSecond),
	})
}

func (s *GardenServer) handleLimitMemory(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-memory", lager.Data{
		"handle": handle,
	})

	var request protocol.LimitMemoryRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	limitInBytes := request.GetLimitInBytes()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	requestedLimits := garden.MemoryLimits{
		LimitInBytes: limitInBytes,
	}

	if request.LimitInBytes != nil {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": requestedLimits,
		})

		err = container.LimitMemory(requestedLimits)

		if err != nil {
			s.writeError(w, err, hLog)
			return
		}
	}

	limits, err := container.CurrentMemoryLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("limited", lager.Data{
		"resulting-limits": limits,
	})

	s.writeResponse(w, &protocol.LimitMemoryResponse{
		LimitInBytes: proto.Uint64(limits.LimitInBytes),
	})
}

func (s *GardenServer) handleCurrentMemoryLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("current-memory-limits", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("getting")

	limits, err := container.CurrentMemoryLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got", lager.Data{
		"limits": limits,
	})

	s.writeResponse(w, &protocol.LimitMemoryResponse{
		LimitInBytes: proto.Uint64(limits.LimitInBytes),
	})
}

func (s *GardenServer) handleLimitDisk(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-disk", lager.Data{
		"handle": handle,
	})

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
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	requestedLimits := garden.DiskLimits{
		BlockSoft: blockSoft,
		BlockHard: blockHard,
		InodeSoft: inodeSoft,
		InodeHard: inodeHard,
		ByteSoft:  byteSoft,
		ByteHard:  byteHard,
	}

	if settingLimit {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": requestedLimits,
		})

		err = container.LimitDisk(requestedLimits)
		if err != nil {
			s.writeError(w, err, hLog)
			return
		}
	}

	limits, err := container.CurrentDiskLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("limited", lager.Data{
		"resulting-limits": limits,
	})

	s.writeResponse(w, &protocol.LimitDiskResponse{
		BlockSoft: proto.Uint64(limits.BlockSoft),
		BlockHard: proto.Uint64(limits.BlockHard),
		InodeSoft: proto.Uint64(limits.InodeSoft),
		InodeHard: proto.Uint64(limits.InodeHard),
		ByteSoft:  proto.Uint64(limits.ByteSoft),
		ByteHard:  proto.Uint64(limits.ByteHard),
	})
}

func (s *GardenServer) handleCurrentDiskLimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("current-disk-limits", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("getting")

	limits, err := container.CurrentDiskLimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got", lager.Data{
		"limits": limits,
	})

	s.writeResponse(w, &protocol.LimitDiskResponse{
		BlockSoft: proto.Uint64(limits.BlockSoft),
		BlockHard: proto.Uint64(limits.BlockHard),
		InodeSoft: proto.Uint64(limits.InodeSoft),
		InodeHard: proto.Uint64(limits.InodeHard),
		ByteSoft:  proto.Uint64(limits.ByteSoft),
		ByteHard:  proto.Uint64(limits.ByteHard),
	})
}

func (s *GardenServer) handleLimitCPU(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-cpu", lager.Data{
		"handle": handle,
	})

	var request protocol.LimitCpuRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	limitInShares := request.GetLimitInShares()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	requestedLimits := garden.CPULimits{
		LimitInShares: limitInShares,
	}

	if request.LimitInShares != nil {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": requestedLimits,
		})

		err = container.LimitCPU(requestedLimits)
		if err != nil {
			s.writeError(w, err, hLog)
			return
		}
	}

	limits, err := container.CurrentCPULimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("limited", lager.Data{
		"resulting-limits": limits,
	})

	s.writeResponse(w, &protocol.LimitCpuResponse{
		LimitInShares: proto.Uint64(limits.LimitInShares),
	})
}

func (s *GardenServer) handleCurrentCPULimits(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("current-cpu-limits", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("getting")

	limits, err := container.CurrentCPULimits()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got", lager.Data{
		"limits": limits,
	})

	s.writeResponse(w, &protocol.LimitCpuResponse{
		LimitInShares: proto.Uint64(limits.LimitInShares),
	})
}

func (s *GardenServer) handleNetIn(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("net-in", lager.Data{
		"handle": handle,
	})

	var request protocol.NetInRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	hostPort := request.GetHostPort()
	containerPort := request.GetContainerPort()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("port-mapping", lager.Data{
		"host-port":      hostPort,
		"container-port": containerPort,
	})

	hostPort, containerPort, err = container.NetIn(hostPort, containerPort)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("port-mapped", lager.Data{
		"host-port":      hostPort,
		"container-port": containerPort,
	})

	s.writeResponse(w, &protocol.NetInResponse{
		HostPort:      proto.Uint32(hostPort),
		ContainerPort: proto.Uint32(containerPort),
	})
}

func validPortRange(portRange string) bool {
	if portRange != "" {
		r := strings.Split(portRange, ":")
		if len(r) != 2 {
			return false
		}
		lo, startErr := strconv.Atoi(r[0])
		hi, endErr := strconv.Atoi(r[1])
		return startErr == nil && endErr == nil && lo > 0 && lo <= 65535 && hi > 0 && hi <= 65535
	}
	return true
}

func (s *GardenServer) handleNetOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("net-out", lager.Data{
		"handle": handle,
	})

	var request protocol.NetOutRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	var protoc garden.Protocol
	switch request.GetProtocol() {
	case protocol.NetOutRequest_TCP:
		protoc = garden.ProtocolTCP
	case protocol.NetOutRequest_ICMP:
		protoc = garden.ProtocolICMP
	case protocol.NetOutRequest_UDP:
		protoc = garden.ProtocolUDP
	case protocol.NetOutRequest_ALL:
		protoc = garden.ProtocolAll
	default:
		err := fmt.Errorf("invalid protocol: %d", request.GetProtocol())
		s.writeError(w, err, hLog)
		return
	}

	network := request.GetNetwork()
	port := request.GetPort()
	portRange := request.GetPortRange()
	icmpType := request.GetIcmpType()
	icmpCode := request.GetIcmpCode()

	if !validPortRange(portRange) {
		err := fmt.Errorf("invalid port range: %q", portRange)
		s.writeError(w, err, hLog)
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("allowing-out", lager.Data{
		"network":   network,
		"port":      port,
		"portRange": portRange,
		"protocol":  protoc,
		"icmpType":  icmpType,
		"icmpCode":  icmpCode,
	})

	err = container.NetOut(network, port, portRange, protoc, icmpType, icmpCode, request.GetLog())
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("allowed", lager.Data{
		"network": network,
		"port":    port,
	})

	s.writeResponse(w, &protocol.NetOutResponse{})
}

func (s *GardenServer) handleGetProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("get-property", lager.Data{
		"handle": handle,
	})

	var request protocol.GetPropertyRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	key := request.GetKey()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("get-property", lager.Data{
		"key": key,
	})

	value, err := container.GetProperty(key)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got-property", lager.Data{
		"key":   key,
		"value": value,
	})

	s.writeResponse(w, &protocol.GetPropertyResponse{
		Value: proto.String(value),
	})
}

func (s *GardenServer) handleSetProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")
	key := r.FormValue(":key")

	hLog := s.logger.Session("set-property", lager.Data{
		"handle": handle,
	})

	var request protocol.SetPropertyRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	value := request.GetValue()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("set-property", lager.Data{
		"key":   key,
		"value": value,
	})

	err = container.SetProperty(key, value)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("set-property-complete", lager.Data{
		"key":   key,
		"value": value,
	})

	s.writeResponse(w, &protocol.SetPropertyResponse{})
}

func (s *GardenServer) handleRemoveProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("remove-property", lager.Data{
		"handle": handle,
	})

	var request protocol.RemovePropertyRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	key := request.GetKey()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("remove-property", lager.Data{
		"key": key,
	})

	err = container.RemoveProperty(key)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("removed-property", lager.Data{
		"key": key,
	})

	s.writeResponse(w, &protocol.RemovePropertyResponse{})
}

func (s *GardenServer) handleRun(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("run", lager.Data{
		"handle": handle,
	})

	var request protocol.RunRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	path := request.GetPath()
	args := request.GetArgs()
	dir := request.GetDir()
	privileged := request.GetPrivileged()
	user := request.GetUser()
	env := request.GetEnv()
	tty := request.GetTty()

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	processSpec := garden.ProcessSpec{
		Path:       path,
		Args:       args,
		Dir:        dir,
		Privileged: privileged,
		User:       user,
		Env:        convertEnv(env),
		TTY:        ttySpecFrom(tty),
	}

	if request.Rlimits != nil {
		processSpec.Limits = resourceLimits(request.Rlimits)
	}

	hLog.Debug("running", lager.Data{
		"spec": processSpec,
	})

	stdout := make(chan []byte, 1000)
	stderr := make(chan []byte, 1000)

	stdinR, stdinW := io.Pipe()

	processIO := garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	process, err := container.Run(processSpec, processIO)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("spawned", lager.Data{
		"spec": processSpec,
		"id":   process.ID(),
	})

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}

	defer conn.Close()

	transport.WriteMessage(conn, &protocol.ProcessPayload{
		ProcessId: proto.Uint32(process.ID()),
	})

	go s.streamInput(json.NewDecoder(br), stdinW, process)

	s.streamProcess(hLog, conn, process, stdout, stderr, stdinW)
}

func (s *GardenServer) handleAttach(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var processID uint32

	hLog := s.logger.Session("attach", lager.Data{
		"handle": handle,
	})

	_, err := fmt.Sscanf(r.FormValue(":pid"), "%d", &processID)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	stdout := make(chan []byte, 1000)
	stderr := make(chan []byte, 1000)

	stdinR, stdinW := io.Pipe()

	processIO := garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	hLog.Debug("attaching", lager.Data{
		"id": processID,
	})

	process, err := container.Attach(processID, processIO)
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}

	hLog.Info("attached", lager.Data{
		"id": process.ID(),
	})

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}

	defer conn.Close()

	go s.streamInput(json.NewDecoder(br), stdinW, process)

	s.streamProcess(hLog, conn, process, stdout, stderr, stdinW)
}

func (s *GardenServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("info", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("getting-info")

	info, err := container.Info()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got-info")

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
		ExternalIp:    proto.String(info.ExternalIP),
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

func resourceLimits(limits *protocol.ResourceLimits) garden.ResourceLimits {
	return garden.ResourceLimits{
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

func (s *GardenServer) writeError(w http.ResponseWriter, err error, logger lager.Logger) {
	logger.Error("failed", err)

	statusCode := http.StatusInternalServerError
	if _, ok := err.(garden.ContainerNotFoundError); ok {
		statusCode = http.StatusNotFound
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(statusCode)
	w.Write([]byte(err.Error()))
}

func (s *GardenServer) writeResponse(w http.ResponseWriter, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	transport.WriteMessage(w, msg)
}

func (s *GardenServer) readRequest(msg proto.Message, w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		s.writeError(w, ErrInvalidContentType, s.logger)
		return false
	}

	err := json.NewDecoder(r.Body).Decode(msg)
	if err != nil {
		s.writeError(w, err, s.logger)
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

func (s *GardenServer) streamInput(decoder *json.Decoder, in *io.PipeWriter, process garden.Process) {
	for {
		var payload protocol.ProcessPayload
		err := decoder.Decode(&payload)
		if err != nil {
			in.CloseWithError(errors.New("Connection closed"))
			return
		}

		switch {
		case payload.Tty != nil:
			process.SetTTY(*ttySpecFrom(payload.GetTty()))

		case payload.Source != nil:
			if payload.Data == nil {
				in.Close()
				return
			} else {
				_, err := in.Write([]byte(payload.GetData()))
				if err != nil {
					return
				}
			}

		case payload.Signal != nil:
			switch payload.GetSignal() {
			case protocol.ProcessPayload_kill:
				process.Signal(garden.SignalKill)
			case protocol.ProcessPayload_terminate:
				process.Signal(garden.SignalTerminate)
			default:
				s.logger.Error("stream-input-unknown-process-payload-signal", nil, lager.Data{"payload": payload})
				in.Close()
				return
			}

		default:
			s.logger.Error("stream-input-unknown-process-payload", nil, lager.Data{"payload": payload})
			in.Close()
			return
		}
	}
}

func (s *GardenServer) streamProcess(logger lager.Logger, conn net.Conn, process garden.Process, stdout <-chan []byte, stderr <-chan []byte, stdinPipe *io.PipeWriter) {
	statusCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		status, err := process.Wait()
		if err != nil {
			logger.Error("wait-failed", err, lager.Data{
				"id": process.ID(),
			})

			errCh <- err
		} else {
			logger.Info("exited", lager.Data{
				"status": status,
				"id":     process.ID(),
			})

			statusCh <- status
		}
	}()

	stdoutSource := protocol.ProcessPayload_stdout
	stderrSource := protocol.ProcessPayload_stderr

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
			flushProcess(conn, process, stdout, stderr)

			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId:  proto.Uint32(process.ID()),
				ExitStatus: proto.Uint32(uint32(status)),
			})

			stdinPipe.Close()
			return

		case err := <-errCh:
			flushProcess(conn, process, stdout, stderr)

			transport.WriteMessage(conn, &protocol.ProcessPayload{
				ProcessId: proto.Uint32(process.ID()),
				Error:     proto.String(err.Error()),
			})

			stdinPipe.Close()
			return

		case <-s.stopping:
			logger.Debug("detaching", lager.Data{
				"id": process.ID(),
			})

			return
		}
	}
}

func flushProcess(conn net.Conn, process garden.Process, stdout <-chan []byte, stderr <-chan []byte) {
	stdoutSource := protocol.ProcessPayload_stdout
	stderrSource := protocol.ProcessPayload_stderr

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

		default:
			return
		}
	}
}

func ttySpecFrom(tty *protocol.TTY) *garden.TTYSpec {
	var ttySpec *garden.TTYSpec
	if tty != nil {
		ttySpec = &garden.TTYSpec{}

		windowSize := tty.GetWindowSize()
		if windowSize != nil {
			ttySpec.WindowSize = &garden.WindowSize{
				Columns: int(windowSize.GetColumns()),
				Rows:    int(windowSize.GetRows()),
			}
		}
	}

	return ttySpec
}
