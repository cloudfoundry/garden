package server

import (
	"net/http"
	"strconv"
	"sync"
)

type StreamServer struct {
	mu      sync.RWMutex
	streams map[uint32]source
	nextID  uint32
}

func NewSteamServer() *StreamServer {
	return &StreamServer{
		streams: make(map[uint32]source),
	}
}

type source struct {
	Stdout chan []byte
	Stderr chan []byte
}

func (m *StreamServer) handleStdout(w http.ResponseWriter, r *http.Request) {
	streamid, err := strconv.Atoi(r.FormValue(":streamid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	stream := uint32(streamid)

	w.WriteHeader(http.StatusOK)

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer conn.Close()

	m.mu.RLock()
	stdoutCh := m.streams[stream].Stdout
	m.mu.RUnlock()

	for {
		if output, ok := <-stdoutCh; ok {
			conn.Write(output)
		} else {
			return
		}
	}
}

func (m *StreamServer) handleStderr(w http.ResponseWriter, r *http.Request) {
	streamid, err := strconv.Atoi(r.FormValue(":streamid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	stream := uint32(streamid)

	w.WriteHeader(http.StatusOK)

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer conn.Close()

	m.mu.RLock()
	stderrCh := m.streams[stream].Stderr
	m.mu.RUnlock()

	for {
		if output, ok := <-stderrCh; ok {
			conn.Write(output)
		} else {
			return
		}
	}
}

func (m *StreamServer) stream(stdout, stderr chan []byte) uint32 {
	m.mu.Lock()
	streamID := m.nextID
	m.nextID++

	m.streams[streamID] = source{
		Stdout: stdout,
		Stderr: stderr,
	}
	m.mu.Unlock()

	return streamID
}
