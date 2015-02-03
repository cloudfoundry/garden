package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/cloudfoundry-incubator/garden"
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

	s.writeSuccess(w)
}

func (s *GardenServer) handleCapacity(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("capacity")

	capacity, err := s.backend.Capacity()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.writeResponse(w, capacity)
}

func (s *GardenServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	var spec garden.ContainerSpec
	if !s.readRequest(&spec, w, r) {
		return
	}

	hLog := s.logger.Session("create", lager.Data{
		"request": spec,
	})

	if spec.GraceTime == 0 {
		spec.GraceTime = s.containerGraceTime
	}

	hLog.Debug("creating")

	container, err := s.backend.Create(spec)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("created")

	s.bomberman.Strap(container)

	s.writeResponse(w, &struct{ Handle string }{
		Handle: container.Handle(),
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

	s.writeResponse(w, &struct{ Handles []string }{handles})
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

	s.writeSuccess(w)
}

func (s *GardenServer) handleStop(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("stop", lager.Data{
		"handle": handle,
	})

	var request struct {
		Kill bool `json:"kill"`
	}
	if !s.readRequest(&request, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("stopping")

	err = container.Stop(request.Kill)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("stopped")

	s.writeSuccess(w)
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

	s.writeSuccess(w)
}

func (s *GardenServer) writeSuccess(w http.ResponseWriter) {
	s.writeResponse(w, &struct{}{})
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

	var request garden.BandwidthLimits
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

	hLog.Debug("limiting", lager.Data{
		"requested-limits": request,
	})

	err = container.LimitBandwidth(request)
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

	s.writeResponse(w, limits)
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

	s.writeResponse(w, limits)
}

func (s *GardenServer) handleLimitMemory(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-memory", lager.Data{
		"handle": handle,
	})

	var request garden.MemoryLimits
	if !s.readRequest(&request, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInBytes > 0 {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": request.LimitInBytes,
		})

		err = container.LimitMemory(request)

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

	s.writeResponse(w, limits)
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

	s.writeResponse(w, limits)
}

func (s *GardenServer) handleLimitDisk(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-disk", lager.Data{
		"handle": handle,
	})

	var request garden.DiskLimits
	if !s.readRequest(&request, w, r) {
		return
	}

	settingLimit := false
	if request.BlockSoft > 0 || request.BlockHard > 0 ||
		request.InodeSoft > 0 || request.InodeHard > 0 ||
		request.ByteSoft > 0 || request.ByteHard > 0 {
		settingLimit = true
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if settingLimit {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": request,
		})

		err = container.LimitDisk(request)
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

	s.writeResponse(w, limits)
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

	s.writeResponse(w, limits)
}

func (s *GardenServer) handleLimitCPU(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("limit-cpu", lager.Data{
		"handle": handle,
	})

	var request garden.CPULimits
	if !s.readRequest(&request, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	if request.LimitInShares > 0 {
		hLog.Debug("limiting", lager.Data{
			"requested-limits": request,
		})

		err = container.LimitCPU(request)
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

	s.writeResponse(w, limits)
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

	s.writeResponse(w, limits)
}

func (s *GardenServer) handleNetIn(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("net-in", lager.Data{
		"handle": handle,
	})

	var request transport.NetInRequest
	if !s.readRequest(&request, w, r) {
		return
	}

	hostPort := request.HostPort
	containerPort := request.ContainerPort

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

	s.writeResponse(w, &transport.NetInResponse{
		HostPort:      hostPort,
		ContainerPort: containerPort,
	})
}

func (s *GardenServer) handleNetOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("net-out", lager.Data{
		"handle": handle,
	})

	var rule garden.NetOutRule
	if !s.readRequest(&rule, w, r) {
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
		"rule": rule,
	})

	err = container.NetOut(rule)

	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Debug("allowed", lager.Data{
		"rule": rule,
	})

	s.writeSuccess(w)
}

func (s *GardenServer) handleGetProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")
	key := r.FormValue(":key")

	hLog := s.logger.Session("get-property", lager.Data{
		"handle": handle,
	})

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

	s.writeResponse(w, map[string]string{
		"value": value,
	})
}

func (s *GardenServer) handleSetProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")
	key := r.FormValue(":key")

	hLog := s.logger.Session("set-property", lager.Data{
		"handle": handle,
	})

	var request struct {
		Value string `json:"value"`
	}
	if !s.readRequest(&request, w, r) {
		return
	}

	value := request.Value

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

	s.writeSuccess(w)
}

func (s *GardenServer) handleRemoveProperty(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")
	key := r.FormValue(":key")

	hLog := s.logger.Session("remove-property", lager.Data{
		"handle": handle,
	})

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

	s.writeSuccess(w)
}

func (s *GardenServer) handleRun(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("run", lager.Data{
		"handle": handle,
	})

	var request garden.ProcessSpec
	if !s.readRequest(&request, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("running", lager.Data{
		"spec": request,
	})

	stdout := make(chan []byte, 1000)
	stderr := make(chan []byte, 1000)

	stdinR, stdinW := io.Pipe()

	processIO := garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	process, err := container.Run(request, processIO)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("spawned", lager.Data{
		"spec": request,
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

	transport.WriteMessage(conn, &transport.ProcessPayload{
		ProcessID: process.ID(),
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

	s.writeResponse(w, info)
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

func (s *GardenServer) writeResponse(w http.ResponseWriter, msg interface{}) {
	w.Header().Set("Content-Type", "application/json")
	transport.WriteMessage(w, msg)
}

func (s *GardenServer) readRequest(msg interface{}, w http.ResponseWriter, r *http.Request) bool {
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

func (s *GardenServer) streamInput(decoder *json.Decoder, in *io.PipeWriter, process garden.Process) {
	for {
		var payload transport.ProcessPayload
		err := decoder.Decode(&payload)
		if err != nil {
			in.CloseWithError(errors.New("Connection closed"))
			return
		}

		switch {
		case payload.TTY != nil:
			process.SetTTY(*payload.TTY)

		case payload.Source != nil:
			if payload.Data == nil {
				in.Close()
				return
			} else {
				_, err := in.Write([]byte(*payload.Data))
				if err != nil {
					return
				}
			}

		case payload.Signal != nil:
			switch *payload.Signal {
			case garden.SignalKill:
				process.Signal(garden.SignalKill)
			case garden.SignalTerminate:
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

	stdoutSource := transport.Stdout
	stderrSource := transport.Stderr

	for {
		select {
		case data := <-stdout:
			d := string(data)
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID: process.ID(),
				Source:    &stdoutSource,
				Data:      &d,
			})

		case data := <-stderr:
			d := string(data)
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID: process.ID(),
				Source:    &stderrSource,
				Data:      &d,
			})

		case status := <-statusCh:
			flushProcess(conn, process, stdout, stderr)

			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID:  process.ID(),
				ExitStatus: &status,
			})

			stdinPipe.Close()
			return

		case err := <-errCh:
			flushProcess(conn, process, stdout, stderr)

			e := err.Error()
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID: process.ID(),
				Error:     &e,
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
	stdoutSource := transport.Stdout
	stderrSource := transport.Stderr

	for {
		select {
		case data := <-stdout:
			d := string(data)
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID: process.ID(),
				Source:    &stdoutSource,
				Data:      &d,
			})

		case data := <-stderr:
			d := string(data)
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID: process.ID(),
				Source:    &stderrSource,
				Data:      &d,
			})

		default:
			return
		}
	}
}
