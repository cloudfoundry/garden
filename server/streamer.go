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
	done    map[uint32]chan struct{}
}

func NewSteamServer() *StreamServer {
	return &StreamServer{
		stdouts: make(map[uint32]chan []byte),
		stderrs: make(map[uint32]chan []byte),
		done:    make(map[uint32]chan struct{}),
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
	done := m.done[stream]
	m.mu.RUnlock()

	for {
		select {
		case output := <-ch:
			conn.Write(output)
		case <-done:
			for {
				select {
				case output := <-ch:
					conn.Write(output)
				default:
					m.mu.Lock()
					defer m.mu.Unlock()
					delete(streams, stream)
					return
				}
			}
		}
	}
}

func (m *StreamServer) stream(stdout, stderr chan []byte) uint32 {
	m.mu.Lock()
	streamID := m.nextID
	m.nextID++

	m.stdouts[streamID] = stdout
	m.stderrs[streamID] = stderr
	m.done[streamID] = make(chan struct{})
	m.mu.Unlock()

	return streamID
}

func (m *StreamServer) stop(id uint32) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	close(m.done[id])
}
