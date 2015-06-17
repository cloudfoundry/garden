package connection

import (
	"fmt"
	"io"
	"sync"

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
		wg:              new(sync.WaitGroup),
	}
}

// attaches to the given standard stream endpoint for a running process
// and copies output to a local io.writer
func (sh *streamHandler) attach(stdtype string) (io.Reader, error) {
	source, err := sh.connect(stdtype)
	if err != nil {
		return nil, err
	}

	sh.wg.Add(1)
	return source, nil
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
