package server

import (
	"io"
	"log"
	"net"

	"code.google.com/p/gogoprotobuf/proto"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/messagereader"
	protocol "github.com/vito/garden/protocol"
)

type WardenServer struct {
	socketPath string
	backend    backend.Backend
}

func New(socketPath string, backend backend.Backend) *WardenServer {
	return &WardenServer{
		socketPath: socketPath,
		backend:    backend,
	}
}

func (s *WardenServer) Start() error {
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}

	go s.handleConnections(listener)

	return nil
}

func (s *WardenServer) handleConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("error accepting connection:", err)
			continue
		}

		go s.serveConnection(conn)
	}
}

func (s *WardenServer) serveConnection(conn net.Conn) {
	for {
		request, err := messagereader.ReadRequest(conn)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Println("error reading request:", err)
			continue
		}

		var response proto.Message

		switch request.(type) {
		case *protocol.PingRequest:
			response = &protocol.PingResponse{}
		case *protocol.EchoRequest:
			response = &protocol.EchoResponse{
				Message: request.(*protocol.EchoRequest).Message,
			}
		default:
			log.Println("unhandled request:", request)
			continue
		}

		protocol.Messages(response).WriteTo(conn)
	}
}
