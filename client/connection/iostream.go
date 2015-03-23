package connection

import (
	"fmt"
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/tedsuo/rata"
)

type ioStream struct {
	*connection
	containerHandle string
	processID       uint32
	streamID        uint32

	wg *sync.WaitGroup
}

func (conn *connection) newIOStream(handle string, processID, streamID uint32) *ioStream {
	return &ioStream{
		connection:      conn,
		containerHandle: handle,
		processID:       processID,
		streamID:        streamID,
	}
}

// attaches to the stdout and stderr endpoints for a running process
// and copies output to a local io.writers
func (a *ioStream) attach(stdoutW, stderrW io.Writer) error {
	params := rata.Params{
		"handle":   a.containerHandle,
		"pid":      fmt.Sprintf("%d", a.processID),
		"streamid": fmt.Sprintf("%d", a.streamID),
	}

	_, stdout, err := a.doHijack(
		routes.Stdout,
		nil,
		params,
		nil,
		"application/json",
	)

	_, stderr, err := a.doHijack(
		routes.Stderr,
		nil,
		params,
		nil,
		"application/json",
	)

	a.wg = new(sync.WaitGroup)
	if stdoutW != nil {
		a.wg.Add(1)
		go func() {
			io.Copy(stdoutW, stdout)
			a.wg.Done()
		}()
	}

	if stderrW != nil {
		a.wg.Add(1)
		go func() {
			io.Copy(stderrW, stderr)
			a.wg.Done()
		}()
	}

	return err
}

func (a *ioStream) wait() {
	a.wg.Wait()
}
