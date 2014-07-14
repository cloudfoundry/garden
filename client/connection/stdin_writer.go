package connection

type stdinWriter struct {
	stream *processStream
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	err := w.stream.WriteStdin(d)
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func (w *stdinWriter) Close() error {
	return w.stream.CloseStdin()
}
