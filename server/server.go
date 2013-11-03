package server

import (
	"io"
	"log"
	"net"

	"github.com/vito/garden/messagereader"
	protocol "github.com/vito/garden/protocol"
)

func Start(socketPath string) error {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}

	go handleConnections(listener)

	return nil
}

func handleConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("error accepting connection:", err)
			continue
		}

		go serveConnection(conn)
	}
}

func serveConnection(conn net.Conn) {
	for {
		request, err := messagereader.ReadRequest(conn)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Println("error reading request:", err)
			continue
		}

		switch request.(type) {
		case *protocol.PingRequest:
			protocol.Messages(&protocol.PingResponse{}).WriteTo(conn)
		case *protocol.EchoRequest:
			protocol.Messages(&protocol.EchoResponse{
				Message: request.(*protocol.EchoRequest).Message,
			}).WriteTo(conn)
		}
	}
}
