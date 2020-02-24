package server

type chanWriter struct {
	ch chan<- []byte
}

func NewChanWriter(ch chan<- []byte) *chanWriter {
	return &chanWriter{ch: ch}
}

func (w *chanWriter) Write(d []byte) (int, error) {
	// prevent buffer reuse from clobbering the data
	data := make([]byte, len(d))
	copy(data, d)

	select {
	case w.ch <- data:
	default:
		// assumption is that writes never block; channel should have buffer to
		// account for slow consumers
	}

	return len(d), nil
}

func (w *chanWriter) Close() error {
	close(w.ch)
	return nil
}
