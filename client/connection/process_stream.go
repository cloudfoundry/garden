package connection

import (
	"fmt"
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/gogo/protobuf/proto"
)

var stdin = protocol.ProcessPayload_stdin
var sigKill = protocol.ProcessPayload_kill
var sigTerm = protocol.ProcessPayload_terminate

type processStream struct {
	id   uint32
	conn net.Conn

	sync.Mutex
}

func (s *processStream) WriteStdin(data []byte) error {
	return s.sendPayload(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(s.id),
		Source:    &stdin,
		Data:      proto.String(string(data)),
	})
}

func (s *processStream) CloseStdin() error {
	return s.sendPayload(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(s.id),
		Source:    &stdin,
	})
}

func (s *processStream) SetTTY(spec garden.TTYSpec) error {
	tty := &protocol.TTY{}

	if spec.WindowSize != nil {
		tty.WindowSize = &protocol.TTY_WindowSize{
			Columns: proto.Uint32(uint32(spec.WindowSize.Columns)),
			Rows:    proto.Uint32(uint32(spec.WindowSize.Rows)),
		}
	}

	return s.sendPayload(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(s.id),
		Tty:       tty,
	})
}

func (s *processStream) Signal(signal garden.Signal) error {
	var payloadSignal protocol.ProcessPayload_Signal
	switch signal {
	case garden.SignalKill:
		payloadSignal = sigKill
	case garden.SignalTerminate:
		payloadSignal = sigTerm
	default:
		return fmt.Errorf("Unknown signal type: %d", signal)
	}

	return s.sendPayload(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(s.id),
		Signal:    &payloadSignal,
	})
}

func (s *processStream) Close() error {
	return s.conn.Close()
}

func (s *processStream) sendPayload(payload *protocol.ProcessPayload) error {
	s.Lock()

	err := transport.WriteMessage(s.conn, payload)
	if err != nil {
		s.Unlock()
		return err
	}

	s.Unlock()

	return nil
}
