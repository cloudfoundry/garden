package server

type chanWriter struct {
	ch chan<- []byte
}

func (w *chanWriter) Write(d []byte) (int, error) {
	// prevent buffer reuse from clobbering the data
	data := make([]byte, len(d))
	copy(data, d)
	w.ch <- data

	return len(d), nil
}

func (w *chanWriter) Close() error {
	close(w.ch)
	return nil
}
