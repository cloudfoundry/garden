package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"code.cloudfoundry.org/garden/transport"
	"code.cloudfoundry.org/lager/v3"
)

type streamHandler struct {
	log        lager.Logger
	wg         *sync.WaitGroup
	writeMutex sync.Mutex
}

func newStreamHandler(log lager.Logger) *streamHandler {
	return &streamHandler{
		log: log,
		wg:  new(sync.WaitGroup),
	}
}

func (sh *streamHandler) streamIn(processWriter io.WriteCloser, stdin io.Reader) {
	if stdin == nil {
		return
	}

	go func(processInputStream io.WriteCloser, stdin io.Reader, log lager.Logger) {
		if _, err := io.Copy(processInputStream, stdin); err == nil {
			err := processInputStream.Close()
			if err != nil {
				sh.log.Debug("failed-to-close-input-stream", lager.Data{"error": err})
			}
		} else {
			log.Error("streaming-stdin-payload", err)
		}
	}(processWriter, stdin, sh.log)
}

func (sh *streamHandler) streamOut(streamWriter io.Writer, streamReader io.Reader) {
	if streamWriter == nil || streamReader == nil {
		sh.log.Debug("nil-stream", lager.Data{
			"streamWriter-nil": streamWriter == nil,
			"streamReader-nil": streamReader == nil,
		})
		return
	}

	sh.wg.Add(1)
	go func() {
		sh.writeMutex.Lock()
		defer sh.writeMutex.Unlock()
		defer sh.wg.Done()

		_, err := io.Copy(streamWriter, streamReader)
		if err != nil {
			sh.log.Debug("failed-to-copy-stream-data", lager.Data{"error": err})
		}
	}()
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
