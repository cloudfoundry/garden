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

type attacher interface {
	attach(stdout, stderr io.Writer) error
	wait()
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

func (p *process) streamPayloads(decoder *json.Decoder, stream attacher, processIO garden.ProcessIO) {
	defer p.stream.Close()

	if processIO.Stdin != nil {
		writer := &stdinWriter{p.stream}

		go func() {
			if _, err := io.Copy(writer, processIO.Stdin); err == nil {
				writer.Close()
			} else {
				p.stream.Close()
			}
		}()
	}

	stream.attach(processIO.Stdout, processIO.Stderr)

	for {
		payload := &transport.ProcessPayload{}

		err := decoder.Decode(payload)
		if err != nil {
			stream.wait()
			p.exited(0, err)
			break
		}

		if payload.Error != nil {
			stream.wait()
			p.exited(0, fmt.Errorf("process error: %s", *payload.Error))
			break
		}

		if payload.ExitStatus != nil {
			stream.wait()
			p.exited(int(*payload.ExitStatus), nil)
			break
		}
	}
}
