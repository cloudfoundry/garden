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

	if err := a.copyStream(streamWriter, stdtype); err != nil {
		return err
	}

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

func (a *ioStream) copyStream(target io.Writer, route string) error {
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
		return fmt.Errorf("Failed to hijack stream %s: %s", route, err)
	}

	a.wg.Add(1)
	go func() {
		io.Copy(target, source)
		a.wg.Done()
	}()

	return nil
}

func (a *ioStream) wait() {
	a.wg.Wait()
}
