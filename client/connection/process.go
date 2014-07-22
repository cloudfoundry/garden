package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type process struct {
	id uint32

	stream *processStream

	done       bool
	exitStatus int
	exitErr    error
	doneL      *sync.Cond
}

func newProcess(id uint32, conn net.Conn) *process {
	return &process{
		id: id,

		stream: &processStream{
			id:   id,
			conn: conn,
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

func (p *process) SetTTY(tty warden.TTYSpec) error {
	return p.stream.SetTTY(tty)
}

func (p *process) exited(exitStatus int, err error) {
	p.doneL.L.Lock()
	p.exitStatus = exitStatus
	p.exitErr = err
	p.done = true
	p.doneL.L.Unlock()

	p.doneL.Broadcast()
}

func (p *process) streamPayloads(decoder *json.Decoder, processIO warden.ProcessIO) {
	defer p.stream.Close()

	if processIO.Stdin != nil {
		writer := &stdinWriter{p.stream}

		go func() {
			io.Copy(writer, processIO.Stdin)
			writer.Close()
		}()
	}

	for {
		payload := &protocol.ProcessPayload{}

		err := decoder.Decode(payload)
		if err != nil {
			p.exited(0, err)
			break
		}

		if payload.Error != nil {
			p.exited(0, fmt.Errorf("process error: %s", payload.GetError()))
			break
		}

		if payload.ExitStatus != nil {
			p.exited(int(payload.GetExitStatus()), nil)
			break
		}

		switch payload.GetSource() {
		case protocol.ProcessPayload_stdout:
			if processIO.Stdout != nil {
				processIO.Stdout.Write([]byte(payload.GetData()))
			}
		case protocol.ProcessPayload_stderr:
			if processIO.Stderr != nil {
				processIO.Stderr.Write([]byte(payload.GetData()))
			}
		}
	}
}
