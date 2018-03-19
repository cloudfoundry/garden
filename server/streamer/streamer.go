package streamer

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// StreamID identifies a pair of standard output and error channels used for streaming.
type StreamID string

// New creates a Streamer with the specified grace time which limits the duration of memory consumption by a stopped stream.
func New(graceTime time.Duration) *Streamer {
	return &Streamer{
		graceTime: graceTime,
		streams:   sync.Map{},
		idGen:     new(syncStreamIDGenerator),
	}
}

type Streamer struct {
	idGen     *syncStreamIDGenerator
	graceTime time.Duration
	streams   sync.Map
}

type stream struct {
	stdout chan []byte
	stderr chan []byte
	done   chan struct{}
}

func newStream(stdout, stderr chan []byte) *stream {
	return &stream{
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
	}
}

// Stream sets up streaming for the given pair of channels and returns a StreamID to identify the pair.
// The caller must call Stop to avoid leaking memory.
func (m *Streamer) Stream(stdout, stderr chan []byte) StreamID {
	sid := m.idGen.next()
	m.streams.Store(sid, newStream(stdout, stderr))

	return sid
}

// StreamStdout streams to the specified writer from the standard output channel of the specified pair of channels.
func (m *Streamer) ServeStdout(streamID StreamID, writer io.Writer) {
	strm, ok := m.loadStream(streamID)
	if !ok {
		return
	}
	m.serve(writer, strm.stdout, strm.done)
}

// StreamStderr streams to the specified writer from the standard error channel of the specified pair of channels.
func (m *Streamer) ServeStderr(streamID StreamID, writer io.Writer) {
	strm, ok := m.loadStream(streamID)
	if !ok {
		return
	}
	m.serve(writer, strm.stderr, strm.done)
}

func (m *Streamer) serve(writer io.Writer, pipe chan []byte, done chan struct{}) {
	for {
		select {
		case b := <-pipe:
			if _, err := writer.Write(b); err != nil {
				return
			}
		case <-done:
			drain(pipe, writer)
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
	strm, ok := m.loadStream(streamID)
	if !ok {
		return
	}

	close(strm.done)

	go func() {
		// wait some time to ensure clients have connected, once they've
		// retrieved the stream from the map it's safe to delete the key
		time.Sleep(m.graceTime)

		m.streams.Delete(streamID)
	}()
}

func (m *Streamer) loadStream(id StreamID) (*stream, bool) {
	strm, ok := m.streams.Load(id)
	if !ok {
		return nil, false
	}
	return strm.(*stream), true
}

type syncStreamIDGenerator struct {
	current uint64
}

func (gen *syncStreamIDGenerator) next() StreamID {
	return StreamID(fmt.Sprintf("%d", atomic.AddUint64(&gen.current, 1)))
}
