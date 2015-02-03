package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/transport"
)

type process struct {
	id uint32

	stream *processStream

	done       bool
	exitStatus int
	exitErr    error
	doneL      *sync.Cond
}

func newProcess(id uint32, netConn net.Conn) *process {
	return &process{
		id: id,

		stream: &processStream{
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
	return p.stream.SetTTY(tty)
}

func (p *process) Signal(signal garden.Signal) error {
	return p.stream.Signal(signal)
}

func (p *process) exited(exitStatus int, err error) {
	p.doneL.L.Lock()
	p.exitStatus = exitStatus
	p.exitErr = err
	p.done = true
	p.doneL.L.Unlock()

	p.doneL.Broadcast()
}

func (p *process) streamPayloads(decoder *json.Decoder, processIO garden.ProcessIO) {
	defer p.stream.Close()

	if processIO.Stdin != nil {
		writer := &stdinWriter{p.stream}

		go func() {
			_, err := io.Copy(writer, processIO.Stdin)
			if err == nil {
				writer.Close()
			} else {
				p.stream.Close()
			}
		}()
	}

	for {
		payload := &transport.ProcessPayload{}

		err := decoder.Decode(payload)
		if err != nil {
			p.exited(0, err)
			break
		}

		if payload.Error != nil {
			p.exited(0, fmt.Errorf("process error: %s", *payload.Error))
			break
		}

		if payload.ExitStatus != nil {
			p.exited(int(*payload.ExitStatus), nil)
			break
		}

		if payload.Source == nil {
			continue
		}

		switch *payload.Source {
		case transport.Stdout:
			if processIO.Stdout != nil {
				processIO.Stdout.Write([]byte(*payload.Data))
			}
		case transport.Stderr:
			if processIO.Stderr != nil {
				processIO.Stderr.Write([]byte(*payload.Data))
			}
		}
	}
}
