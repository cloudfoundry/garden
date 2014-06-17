package connection_test

import (
	"bufio"
	"net"

	"code.google.com/p/goprotobuf/proto"

	"github.com/cloudfoundry-incubator/garden/transport"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type DummyServer struct {
	ReceivedMessages chan proto.Message
	MessagesToSend   chan proto.Message

	closeConnectionChan chan struct{}
	stopReadingChan     chan struct{}
	addr                net.Addr
}

func NewDummyServer() *DummyServer {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	Ω(err).ShouldNot(HaveOccurred())

	server := &DummyServer{
		MessagesToSend:   make(chan proto.Message, 100),
		ReceivedMessages: make(chan proto.Message),

		closeConnectionChan: make(chan struct{}),
		stopReadingChan:     make(chan struct{}),
		addr:                listener.Addr(),
	}

	go func() {
		defer GinkgoRecover()
		conn, err := listener.Accept()
		Ω(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			reader := bufio.NewReader(conn)
			for {
				req, err := transport.ReadRequest(reader)

				if err != nil {
					break
				}

				server.ReceivedMessages <- req

				select {
				case <-server.stopReadingChan:
					break
				default:
				}
			}
		}()

		go func() {
			defer GinkgoRecover()
			for {
				msg, ok := <-server.MessagesToSend
				if !ok {
					break
				}
				transport.WriteMessage(conn, msg)
			}
		}()

		<-server.closeConnectionChan
		err = conn.Close()
		Ω(err).ShouldNot(HaveOccurred())
		err = listener.Close()
		Ω(err).ShouldNot(HaveOccurred())
	}()

	return server
}

func (s *DummyServer) StopReading() {
	close(s.stopReadingChan)
}

func (s *DummyServer) Addr() net.Addr {
	return s.addr
}

func (s *DummyServer) Close() {
	if s.closeConnectionChan != nil {
		s.closeConnectionChan <- struct{}{}
		close(s.MessagesToSend)
		s.closeConnectionChan = nil
	}
}
