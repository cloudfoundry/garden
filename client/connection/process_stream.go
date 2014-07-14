package connection

import (
	"net"
	"sync"

	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
)

var stdin = protocol.ProcessPayload_stdin

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

func (s *processStream) SetWindowSize(columns int, rows int) error {
	return s.sendPayload(&protocol.ProcessPayload{
		ProcessId: proto.Uint32(s.id),
		WindowSize: &protocol.ProcessPayload_WindowSize{
			Columns: proto.Uint32(uint32(columns)),
			Rows:    proto.Uint32(uint32(rows)),
		},
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
