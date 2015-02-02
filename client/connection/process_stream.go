package connection

import (
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/transport"
)

type processStream struct {
	id   uint32
	conn net.Conn

	sync.Mutex
}

func (s *processStream) WriteStdin(data []byte) error {
	d := string(data)
	stdin := transport.Stdin
	return s.sendPayload(transport.ProcessPayload{
		ProcessID: s.id,
		Source:    &stdin,
		Data:      &d,
	})
}

func (s *processStream) CloseStdin() error {
	stdin := transport.Stdin
	return s.sendPayload(transport.ProcessPayload{
		ProcessID: s.id,
		Source:    &stdin,
	})
}

func (s *processStream) SetTTY(spec garden.TTYSpec) error {
	return s.sendPayload(&transport.ProcessPayload{
		ProcessID: s.id,
		TTY:       &spec,
	})
}

func (s *processStream) Signal(signal garden.Signal) error {
	return s.sendPayload(&transport.ProcessPayload{
		ProcessID: s.id,
		Signal:    &signal,
	})
}

func (s *processStream) Close() error {
	return s.conn.Close()
}

func (s *processStream) sendPayload(payload interface{}) error {
	s.Lock()

	err := transport.WriteMessage(s.conn, payload)
	if err != nil {
		s.Unlock()
		return err
	}

	s.Unlock()

	return nil
}
