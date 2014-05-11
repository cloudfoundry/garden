package releasenotifier

import "io"

type ReleaseNotifier struct {
	io.Reader
	io.WriteCloser

	Callback func() error
}

func (notifier ReleaseNotifier) Close() error {
	err := notifier.WriteCloser.Close()
	if err != nil {
		return err
	}

	return notifier.Callback()
}

func (notifier ReleaseNotifier) Read(buf []byte) (int, error) {
	n, err := notifier.Reader.Read(buf)
	if err != nil {
		notifier.Callback()
	}

	return n, err
}
