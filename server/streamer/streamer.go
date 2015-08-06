package streamer

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// StreamID identifies a pair of standard output and error channels used for streaming.
type StreamID string

// New creates a Streamer with the specified grace time which limits the duration of memory consumption by a stopped stream.
func New(graceTime time.Duration) *Streamer {
	return &Streamer{
		graceTime: graceTime,
		streams:   make(map[StreamID]*stream),
	}
}

type Streamer struct {
	mu           sync.RWMutex
	nextStreamID uint64
	graceTime    time.Duration
	streams      map[StreamID]*stream
}

type stream struct {
	ch   [2]chan []byte
	done chan struct{}
}

type stdoutOrErr int

const (
	stdout stdoutOrErr = 0
	stderr stdoutOrErr = 1
)

// Stream sets up streaming for the given pair of channels and returns a StreamID to identify the pair.
// The caller must call Stop to avoid leaking memory.
func (m *Streamer) Stream(stdout, stderr chan []byte) StreamID {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sid StreamID = StreamID(fmt.Sprintf("%d", m.nextStreamID))
	m.nextStreamID++

	m.streams[sid] = &stream{
		ch:   [2]chan []byte{stdout, stderr},
		done: make(chan struct{}),
	}

	return sid
}

// StreamStdout streams to the specified writer from the standard output channel of the specified pair of channels.
func (m *Streamer) ServeStdout(streamID StreamID, writer io.Writer) {
	m.serve(streamID, writer, stdout)
}

// StreamStderr streams to the specified writer from the standard error channel of the specified pair of channels.
func (m *Streamer) ServeStderr(streamID StreamID, writer io.Writer) {
	m.serve(streamID, writer, stderr)
}

func (m *Streamer) serve(streamID StreamID, writer io.Writer, chanIndex stdoutOrErr) {
	strm := m.streamFromID(streamID)

	ch := strm.ch[chanIndex]
	for {
		select {
		case b := <-ch:
			if _, err := writer.Write(b); err != nil {
				return
			}
		case <-strm.done:
			drain(ch, writer)
			return
		}
	}
}

func drain(ch chan []byte, writer io.Writer) {
	for {
		select {
		case b := <-ch:
			writer.Write(b)
		default:
			return
		}
	}
}

// Stop stops streaming from the specified pair of channels.
func (m *Streamer) Stop(streamID StreamID) {
	strm := m.streamFromID(streamID)
	close(strm.done)

	go func() {
		// wait some time to ensure clients have connected, once they've
		// retrieved the stream from the map it's safe to delete the key
		time.Sleep(m.graceTime)

		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.streams, streamID)
	}()
}

func (m *Streamer) streamFromID(streamID StreamID) *stream {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.streams[streamID]
}
