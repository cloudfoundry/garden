package server

import (
	"net/http"
	"strconv"
	"sync"
)

type StreamServer struct {
	mu     sync.RWMutex
	nextID uint32

	stdouts map[uint32]chan []byte
	stderrs map[uint32]chan []byte
}

func NewSteamServer() *StreamServer {
	return &StreamServer{
		stdouts: make(map[uint32]chan []byte),
		stderrs: make(map[uint32]chan []byte),
	}
}

func (m *StreamServer) handleStdout(w http.ResponseWriter, r *http.Request) {
	m.handleStream(w, r, m.stdouts)
}

func (m *StreamServer) handleStderr(w http.ResponseWriter, r *http.Request) {
	m.handleStream(w, r, m.stderrs)
}

func (m *StreamServer) handleStream(w http.ResponseWriter, r *http.Request, streams map[uint32]chan []byte) {
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
	ch := streams[stream]
	m.mu.RUnlock()

	for {
		if output, ok := <-ch; ok {
			conn.Write(output)
		} else {
			m.mu.Lock()
			defer m.mu.Unlock()
			delete(streams, stream)
			return
		}
	}
}

func (m *StreamServer) stream(stdout, stderr chan []byte) uint32 {
	m.mu.Lock()
	streamID := m.nextID
	m.nextID++

	m.stdouts[streamID] = stdout
	m.stderrs[streamID] = stderr
	m.mu.Unlock()

	return streamID
}
