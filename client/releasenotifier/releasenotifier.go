package releasenotifier

import "io"

type ReleaseNotifier struct {
	io.Reader
	io.WriteCloser

	Callback func()
}

func (notifier ReleaseNotifier) Close() error {
	notifier.Callback()
	return notifier.WriteCloser.Close()
}

func (notifier ReleaseNotifier) Read(buf []byte) (int, error) {
	n, err := notifier.Reader.Read(buf)
	if err != nil {
		notifier.Callback()
	}

	return n, err
}
