package fake_backend

import (
	"io"
	"sync"
)

type CloseTracker struct {
	io.Reader
	io.Writer

	closed      chan struct{}
	closedMutex *sync.RWMutex
}

func (tracker *CloseTracker) Close() error {
	close(tracker.closed)
	return nil
}

func (tracker *CloseTracker) IsClosed() bool {
	select {
	case <-tracker.closed:
		return true
	default:
		return false
	}
}
