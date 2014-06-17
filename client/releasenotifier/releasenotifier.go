package releasenotifier

import "io"

type ReleaseNotifier struct {
	io.Reader
	io.WriteCloser

	WriteCallback func() error
	CloseCallback func() error
}

func (notifier ReleaseNotifier) Close() error {
	err := notifier.WriteCloser.Close()
	if err != nil {
		return err
	}

	return notifier.CloseCallback()
}

func (notifier ReleaseNotifier) Write(buf []byte) (int, error) {
	if notifier.WriteCallback != nil {
		if err := notifier.WriteCallback(); err != nil {
			return 0, err
		}
	}
	return notifier.WriteCloser.Write(buf)
}

func (notifier ReleaseNotifier) Read(buf []byte) (int, error) {
	n, err := notifier.Reader.Read(buf)
	if err != nil {
		notifier.CloseCallback()
	}

	return n, err
}
