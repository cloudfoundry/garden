package connection

import (
	"fmt"
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/tedsuo/rata"
)

type streamHandler struct {
	conn            *connection
	containerHandle string
	processID       uint32
	streamID        uint32

	wg *sync.WaitGroup
}

func newStreamHandler(conn *connection, handle string, processID, streamID uint32) *streamHandler {
	return &streamHandler{
		conn:            conn,
		containerHandle: handle,
		processID:       processID,
		streamID:        streamID,
	}
}

func (sh *streamHandler) doAttach(streamWriter io.Writer, stdtype string) error {
	if streamWriter == nil {
		return nil
	}

	source, err := sh.connect(stdtype)
	if err != nil {
		return err
	}

	sh.wg.Add(1)
	go sh.copyStream(streamWriter, source)
	return nil
}

// attaches to the stdout and stderr endpoints for a running process
// and copies output to a local io.writers
func (sh *streamHandler) attach(stdoutW, stderrW io.Writer) error {
	sh.wg = new(sync.WaitGroup)

	if err := sh.doAttach(stdoutW, routes.Stdout); err != nil {
		return err
	}

	if err := sh.doAttach(stderrW, routes.Stderr); err != nil {
		return err
	}

	return nil
}

func (sh *streamHandler) connect(route string) (io.Reader, error) {
	params := rata.Params{
		"handle":   sh.containerHandle,
		"pid":      fmt.Sprintf("%d", sh.processID),
		"streamid": fmt.Sprintf("%d", sh.streamID),
	}
	_, source, err := sh.conn.doHijack(
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

func (sh *streamHandler) copyStream(target io.Writer, source io.Reader) {
	io.Copy(target, source)
	sh.wg.Done()
}

func (sh *streamHandler) wait() {
	sh.wg.Wait()
}
