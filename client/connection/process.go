package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/pivotal-golang/lager"
)

type process struct {
	id uint32

	processInputStream *processStream
	conn               net.Conn
	done               bool
	exitStatus         int
	exitErr            error
	doneL              *sync.Cond
}

type attacher interface {
	attach(streamType string) (io.Reader, error)
	copyStream(target io.Writer, source io.Reader)
	wait()
}

func newProcess(id uint32, netConn net.Conn) *process {
	return &process{
		id:   id,
		conn: netConn,
		processInputStream: &processStream{
			id:   id,
			conn: netConn,
		},

		doneL: sync.NewCond(&sync.Mutex{}),
	}
}

func (p *process) ID() uint32 {
	return p.id
}

func (p *process) Wait() (int, error) {
	p.doneL.L.Lock()

	for !p.done {
		p.doneL.Wait()
	}

	defer p.doneL.L.Unlock()

	return p.exitStatus, p.exitErr
}

func (p *process) SetTTY(tty garden.TTYSpec) error {
	return p.processInputStream.SetTTY(tty)
}

func (p *process) Signal(signal garden.Signal) error {
	return p.processInputStream.Signal(signal)
}

func (p *process) exited(exitStatus int, err error) {
	p.doneL.L.Lock()
	p.exitStatus = exitStatus
	p.exitErr = err
	p.done = true
	p.doneL.L.Unlock()

	p.doneL.Broadcast()
}

func (p *process) streamIn(log lager.Logger, stdin io.Reader) {
	if stdin != nil {
		processInputStreamWriter := &stdinWriter{p.processInputStream}
		if _, err := io.Copy(processInputStreamWriter, stdin); err == nil {
			processInputStreamWriter.Close()
		} else {
			log.Error("streaming-stdin-payload", err)
		}
	}
}

func (p *process) streamOut(streamType string, streamWriter io.Writer, streamHandler attacher, log lager.Logger) {
	errorf := func(err error, streamType string, log lager.Logger) {
		connectionErr := fmt.Errorf("connection: attach to stream %s: %s", streamType, err)
		p.exited(0, connectionErr)
		log.Error("attach-to-stream-failed", connectionErr)
	}

	if streamWriter != nil {
		stdout, err := streamHandler.attach(streamType)
		if err != nil {
			errorf(err, streamType, log)
			return
		}
		go streamHandler.copyStream(streamWriter, stdout)
	}
}

func (p *process) wait(decoder *json.Decoder, streamHandler attacher) {
	defer p.conn.Close()

	for {
		payload := &transport.ProcessPayload{}
		err := decoder.Decode(payload)
		if err != nil {
			streamHandler.wait()
			p.exited(0, fmt.Errorf("connection: decode failed: %s", err))
			break
		}

		if payload.Error != nil {
			streamHandler.wait()
			p.exited(0, fmt.Errorf("connection: process error: %s", *payload.Error))
			break
		}

		if payload.ExitStatus != nil {
			streamHandler.wait()
			p.exited(int(*payload.ExitStatus), nil)
			break
		}

		// discard other payloads
	}
}
