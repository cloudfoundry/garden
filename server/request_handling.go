package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/transport"
	"code.cloudfoundry.org/lager"
)

type processDebugInfo struct {
	Path   string
	Dir    string
	User   string
	Limits garden.ResourceLimits
	TTY    *garden.TTYSpec
}

type containerDebugInfo struct {
	Handle     string
	GraceTime  time.Duration
	RootFSPath string
	BindMounts []garden.BindMount
	Network    string
	Privileged bool
	Limits     garden.Limits
}

var ErrConcurrentDestroy = errors.New("container already being destroyed")

func (s *GardenServer) handlePing(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("ping")

	err := s.backend.Ping()
	if err != nil {
		s.writeError(w, err, hLog)
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
		"request": containerDebugInfo{
			Handle:     spec.Handle,
			GraceTime:  spec.GraceTime,
			RootFSPath: spec.RootFSPath,
			BindMounts: spec.BindMounts,
			Network:    spec.Network,
			Privileged: spec.Privileged,
			Limits:     spec.Limits,
		},
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

	hLog := s.logger.Session("list")
	hLog.Debug("started")

	containers, err := s.backend.Containers(properties)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	handles := []string{}

	for _, container := range containers {
		handles = append(handles, container.Handle())
	}

	hLog.Debug("ending", lager.Data{"handles": handles})

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

	user := r.URL.Query().Get("user")
	dstPath := r.URL.Query().Get("destination")

	hLog := s.logger.Session("stream-in", lager.Data{
		"handle":      handle,
		"user":        user,
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

	err = container.StreamIn(garden.StreamInSpec{
		User:      user,
		Path:      dstPath,
		TarStream: r.Body,
	})
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

	user := r.URL.Query().Get("user")
	srcPath := r.URL.Query().Get("source")

	hLog := s.logger.Session("stream-out", lager.Data{
		"handle": handle,
		"user":   user,
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

	reader, err := container.StreamOut(garden.StreamOutSpec{
		User: user,
		Path: srcPath,
	})
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

func (s *GardenServer) handleBulkNetOut(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("bulk-net-out", lager.Data{
		"handle": handle,
	})

	var rules []garden.NetOutRule
	if !s.readRequest(&rules, w, r) {
		return
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("allowing-bulk-out", lager.Data{
		"rules": rules,
	})

	err = container.BulkNetOut(rules)

	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Debug("allowed", lager.Data{
		"rules": rules,
	})

	s.writeSuccess(w)
}

func (s *GardenServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("get-metrics", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	metrics, err := container.Metrics()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.writeResponse(w, metrics)
}

func (s *GardenServer) handleProperties(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("get-properties", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	properties, err := container.Properties()
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got-properties")

	s.writeResponse(w, properties)
}

func (s *GardenServer) handleProperty(w http.ResponseWriter, r *http.Request) {
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

	hLog.Debug("get-property", lager.Data{})

	value, err := container.Property(key)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Debug("got-property", lager.Data{})

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

	hLog.Debug("set-property", lager.Data{})

	err = container.SetProperty(key, value)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Debug("set-property-complete", lager.Data{})

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

	hLog.Debug("remove-property", lager.Data{})

	err = container.RemoveProperty(key)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("removed-property", lager.Data{})

	s.writeSuccess(w)
}

func (s *GardenServer) handleSetGraceTime(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	var graceTime time.Duration
	if !s.readRequest(&graceTime, w, r) {
		return
	}

	hLog := s.logger.Session("set-grace-time", lager.Data{
		"handle": handle,
	})

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	container.SetGraceTime(graceTime)

	s.bomberman.Reset(container)

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

	info := processDebugInfo{
		Path:   request.Path,
		Dir:    request.Dir,
		User:   request.User,
		Limits: request.Limits,
		TTY:    request.TTY,
	}

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	hLog.Debug("running", lager.Data{
		"spec": info,
	})

	stdout := make(chan []byte, 4000)
	stderr := make(chan []byte, 4000)

	stdinR, stdinW := io.Pipe()

	processIO := garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: &chanWriter{stdout},
		Stderr: &chanWriter{stderr},
	}

	process, err := container.Run(request, processIO)
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}
	hLog.Info("spawned", lager.Data{
		"spec": info,
		"id":   process.ID(),
	})

	streamID := s.streamer.Stream(stdout, stderr)
	defer s.streamer.Stop(streamID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}

	defer conn.Close()

	transport.WriteMessage(conn, &transport.ProcessPayload{
		ProcessID: process.ID(),
		StreamID:  string(streamID),
	})

	connCloseCh := make(chan struct{}, 1)

	go s.streamInput(json.NewDecoder(br), stdinW, process, connCloseCh)

	s.streamProcess(hLog, conn, process, stdinW, connCloseCh)
}

func (s *GardenServer) handleAttach(w http.ResponseWriter, r *http.Request) {
	handle := r.FormValue(":handle")

	hLog := s.logger.Session("attach", lager.Data{
		"handle": handle,
	})

	processID := r.FormValue(":pid")

	container, err := s.backend.Lookup(handle)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	s.bomberman.Pause(container.Handle())
	defer s.bomberman.Unpause(container.Handle())

	stdout := make(chan []byte, 4000)
	stderr := make(chan []byte, 4000)

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

	streamID := s.streamer.Stream(stdout, stderr)
	defer s.streamer.Stop(streamID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	conn, br, err := w.(http.Hijacker).Hijack()
	if err != nil {
		s.writeError(w, err, hLog)
		stdinW.Close()
		return
	}

	defer conn.Close()

	transport.WriteMessage(conn, &transport.ProcessPayload{
		ProcessID: process.ID(),
		StreamID:  string(streamID),
	})

	connCloseCh := make(chan struct{}, 1)

	go s.streamInput(json.NewDecoder(br), stdinW, process, connCloseCh)

	s.streamProcess(hLog, conn, process, stdinW, connCloseCh)
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

func (s *GardenServer) handleBulkInfo(w http.ResponseWriter, r *http.Request) {
	handles := splitHandles(r.URL.Query()["handles"][0])

	hLog := s.logger.Session("bulk_info", lager.Data{
		"handles": handles,
	})
	hLog.Debug("getting-bulkinfo")

	bulkInfo, err := s.backend.BulkInfo(handles)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got-bulkinfo")

	s.writeResponse(w, bulkInfo)
}

func (s *GardenServer) handleBulkMetrics(w http.ResponseWriter, r *http.Request) {
	handles := splitHandles(r.URL.Query()["handles"][0])

	hLog := s.logger.Session("bulk_metrics", lager.Data{
		"handles": handles,
	})
	hLog.Debug("getting-bulkmetrics")

	bulkMetrics, err := s.backend.BulkMetrics(handles)
	if err != nil {
		s.writeError(w, err, hLog)
		return
	}

	hLog.Info("got-bulkmetrics")

	s.writeResponse(w, bulkMetrics)
}

func isClientError(err error) bool {
	if _, ok := err.(garden.ProcessNotFoundError); ok {
		return true
	}

	return false
}

func (s *GardenServer) writeError(w http.ResponseWriter, err error, logger lager.Logger) {
	if !isClientError(err) {
		logger.Error("failed", err)
	}

	w.Header().Set("Content-Type", "application/json")
	merr := &garden.Error{Err: err}

	w.WriteHeader(merr.StatusCode())
	json.NewEncoder(w).Encode(merr)
}

func (s *GardenServer) writeResponse(w http.ResponseWriter, msg interface{}) {
	w.Header().Set("Content-Type", "application/json")
	transport.WriteMessage(w, msg)
}

func (s *GardenServer) readRequest(msg interface{}, w http.ResponseWriter, r *http.Request) bool {
	err := json.NewDecoder(r.Body).Decode(msg)
	if err != nil {
		s.writeError(w, err, s.logger)
		return false
	}

	return true
}

func (s *GardenServer) streamInput(decoder *json.Decoder, in *io.PipeWriter, process garden.Process, connCloseCh chan struct{}) {
	for {
		var payload transport.ProcessPayload
		err := decoder.Decode(&payload)
		if err != nil {
			close(connCloseCh)
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
			s.logger.Info("stream-input-process-signal", lager.Data{"payload": payload})

			switch *payload.Signal {
			case garden.SignalKill:
				err = process.Signal(garden.SignalKill)
				if err != nil {
					s.logger.Error("stream-input-process-signal-kill-failed", err, lager.Data{"payload": payload})
				}
			case garden.SignalTerminate:
				err = process.Signal(garden.SignalTerminate)
				if err != nil {
					s.logger.Error("stream-input-process-signal-terminate-failed", err, lager.Data{"payload": payload})
				}
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

func (s *GardenServer) streamProcess(logger lager.Logger, conn net.Conn, process garden.Process, stdinPipe *io.PipeWriter, connCloseCh chan struct{}) {
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

	for {
		select {

		case status := <-statusCh:
			transport.WriteMessage(conn, &transport.ProcessPayload{
				ProcessID:  process.ID(),
				ExitStatus: &status,
			})

			stdinPipe.Close()
			return

		case err := <-errCh:
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

		case <-connCloseCh:

			return
		}
	}
}

func splitHandles(queryHandles string) []string {
	handles := []string{}
	if queryHandles != "" {
		handles = strings.Split(queryHandles, ",")
	}
	return handles
}
