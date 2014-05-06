package fake_backend

import "io"

type CloseTracker struct {
	io.Reader
	io.Writer

	closed chan struct{}
}

func NewCloseTracker(reader io.Reader, writer io.Writer) *CloseTracker {
	return &CloseTracker{
		Reader: reader,
		Writer: writer,

		closed: make(chan struct{}),
	}
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
