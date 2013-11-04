package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

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
		case *protocol.CreateRequest:
			createRequest := request.(*protocol.CreateRequest)

			bindMounts := []backend.BindMount{}

			for _, bm := range createRequest.GetBindMounts() {
				bindMount := backend.BindMount{
					SrcPath: bm.GetSrcPath(),
					DstPath: bm.GetDstPath(),
					Mode:    backend.BindMountMode(bm.GetMode()),
				}

				bindMounts = append(bindMounts, bindMount)
			}

			container, err := s.backend.Create(backend.ContainerSpec{
				Handle:     createRequest.GetHandle(),
				GraceTime:  time.Duration(createRequest.GetGraceTime()) * time.Second,
				RootFSPath: createRequest.GetRootfs(),
				Network:    createRequest.GetNetwork(),
				BindMounts: bindMounts,
			})

			if err != nil {
				response = errorResponse(err)
				break
			}

			response = &protocol.CreateResponse{
				Handle: proto.String(container.Handle()),
			}
		case *protocol.DestroyRequest:
			handle := request.(*protocol.DestroyRequest).GetHandle()

			err := s.backend.Destroy(handle)
			if err != nil {
				response = errorResponse(err)
				break
			}

			response = &protocol.DestroyResponse{}
		case *protocol.ListRequest:
			containers, err := s.backend.Containers()
			if err != nil {
				response = errorResponse(err)
				break
			}

			handles := []string{}

			for _, container := range containers {
				handles = append(handles, container.Handle())
			}

			response = &protocol.ListResponse{
				Handles: handles,
			}
		case *protocol.CopyInRequest:
			copyIn := request.(*protocol.CopyInRequest)

			handle := copyIn.GetHandle()
			srcPath := copyIn.GetSrcPath()
			dstPath := copyIn.GetDstPath()

			container, err := s.backend.Lookup(handle)
			if err != nil {
				response = errorResponse(err)
				break
			}

			err = container.CopyIn(srcPath, dstPath)
			if err != nil {
				response = errorResponse(err)
				break
			}

			response = &protocol.CopyInResponse{}
		case *protocol.CopyOutRequest:
			copyOut := request.(*protocol.CopyOutRequest)

			handle := copyOut.GetHandle()
			srcPath := copyOut.GetSrcPath()
			dstPath := copyOut.GetDstPath()
			owner := copyOut.GetOwner()

			container, err := s.backend.Lookup(handle)
			if err != nil {
				response = errorResponse(err)
				break
			}

			err = container.CopyOut(srcPath, dstPath, owner)
			if err != nil {
				response = errorResponse(err)
				break
			}

			response = &protocol.CopyOutResponse{}
		}

		if response == nil {
			response = &protocol.ErrorResponse{
				Message: proto.String(fmt.Sprintf("unhandled request type: %T", request)),
			}
		}

		protocol.Messages(response).WriteTo(conn)
	}
}

func errorResponse(err error) proto.Message {
	return &protocol.ErrorResponse{
		Message: proto.String(err.Error()),
	}
}
