package connection

import (
	"net"

	"code.google.com/p/goprotobuf/proto"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type stdinWriter struct {
	process warden.Process
	conn    net.Conn
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	stdin := protocol.ProcessPayload_stdin

	err := transport.WriteMessage(w.conn, &protocol.ProcessPayload{
		ProcessId: proto.Uint32(w.process.ID()),
		Source:    &stdin,
		Data:      proto.String(string(d)),
	})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func (w *stdinWriter) Close() error {
	stdin := protocol.ProcessPayload_stdin

	return transport.WriteMessage(w.conn, &protocol.ProcessPayload{
		ProcessId: proto.Uint32(w.process.ID()),
		Source:    &stdin,
	})
}
