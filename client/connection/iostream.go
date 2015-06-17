package connection

import (
	"fmt"
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/tedsuo/rata"
)

type ioStream struct {
	conn            *connection
	containerHandle string
	processID       uint32
	streamID        uint32

	wg *sync.WaitGroup
}

func newIOStream(conn *connection, handle string, processID, streamID uint32) *ioStream {
	return &ioStream{
		conn:            conn,
		containerHandle: handle,
		processID:       processID,
		streamID:        streamID,
	}
}

func (a *ioStream) doAttach(streamWriter io.Writer, stdtype string) error {
	if streamWriter == nil {
		return nil
	}

	source, err := a.connect(stdtype)
	if err != nil {
		return err
	}

	a.wg.Add(1)
	go a.copyStream(streamWriter, source)
	return nil
}

// attaches to the stdout and stderr endpoints for a running process
// and copies output to a local io.writers
func (a *ioStream) attach(stdoutW, stderrW io.Writer) error {
	a.wg = new(sync.WaitGroup)

	if err := a.doAttach(stdoutW, routes.Stdout); err != nil {
		return err
	}

	if err := a.doAttach(stderrW, routes.Stderr); err != nil {
		return err
	}

	return nil
}

func (a *ioStream) connect(route string) (io.Reader, error) {
	params := rata.Params{
		"handle":   a.containerHandle,
		"pid":      fmt.Sprintf("%d", a.processID),
		"streamid": fmt.Sprintf("%d", a.streamID),
	}
	_, source, err := a.conn.doHijack(
		route,
		nil,
		params,
		nil,
		"application/json",
	)

	if err != nil {
		return nil, fmt.Errorf("Failed to hijack stream %s: %s", route, err)
	}

	return source, nil
}

func (a *ioStream) copyStream(target io.Writer, source io.Reader) {
	io.Copy(target, source)
	a.wg.Done()
}

func (a *ioStream) wait() {
	a.wg.Wait()
}
