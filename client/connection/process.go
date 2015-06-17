package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/routes"
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
	attach(stdtype string) (io.Reader, error)
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

func (p *process) streamIn(log lager.Logger, processIO garden.ProcessIO) {
	if processIO.Stdin != nil {
		processInputStreamWriter := &stdinWriter{p.processInputStream}
		if _, err := io.Copy(processInputStreamWriter, processIO.Stdin); err == nil {
			processInputStreamWriter.Close()
		} else {
			log.Error("streaming-stdin-payload", err)
		}
	}
}

func (p *process) streamOutErr(log lager.Logger, decoder *json.Decoder, streamHandler attacher, processIO garden.ProcessIO) {
	defer p.conn.Close()
	errorf := func(err error, stdtype string) {
		werr := fmt.Errorf("connection: attach to stream %s: %s", stdtype, err)
		p.exited(0, werr)
		log.Error("attach-to-stream-failed", werr)
	}

	if processIO.Stdout != nil {
		if stdout, err := streamHandler.attach(routes.Stdout); err != nil {
			errorf(err, routes.Stdout)
			return
		} else {
			go streamHandler.copyStream(processIO.Stdout, stdout)
		}
	}

	if processIO.Stderr != nil {
		if stderr, err := streamHandler.attach(routes.Stderr); err != nil {
			errorf(err, routes.Stderr)
			return
		} else {
			go streamHandler.copyStream(processIO.Stderr, stderr)
		}
	}

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
