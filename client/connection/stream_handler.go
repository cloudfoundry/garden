package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/pivotal-golang/lager"
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

func (p *streamHandler) streamIn(inputStream *processStream, stdin io.Reader, log lager.Logger) {
	if stdin != nil {
		processInputStreamWriter := &stdinWriter{inputStream}
		if _, err := io.Copy(processInputStreamWriter, stdin); err == nil {
			processInputStreamWriter.Close()
		} else {
			log.Error("streaming-stdin-payload", err)
		}
	}
}

func (sh *streamHandler) streamOut(streamType string, streamWriter io.Writer, log lager.Logger) error {
	if streamWriter == nil {
		return nil
	}

	if stdout, err := sh.attach(streamType); err != nil {
		err := fmt.Errorf("connection: attach to stream %s: %s", streamType, err)
		log.Error("attach-to-stream-failed", err)
		return err
	} else {
		go sh.copyStream(streamWriter, stdout)
	}

	return nil
}

// attaches to the given standard stream endpoint for a running process
// and copies output to a local io.writer
func (sh *streamHandler) attach(streamType string) (io.Reader, error) {
	source, err := sh.connect(streamType)
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

func (sh *streamHandler) wait(decoder *json.Decoder) (int, error) {
	for {
		payload := &transport.ProcessPayload{}
		err := decoder.Decode(payload)
		if err != nil {
			sh.wg.Wait()
			return 0, fmt.Errorf("connection: decode failed: %s", err)
		}

		if payload.Error != nil {
			sh.wg.Wait()
			return 0, fmt.Errorf("connection: process error: %s", *payload.Error)
		}

		if payload.ExitStatus != nil {
			sh.wg.Wait()
			status := int(*payload.ExitStatus)
			return status, nil
		}

		// discard other payloads
	}
}
