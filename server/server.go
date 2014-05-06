package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"code.google.com/p/gogoprotobuf/proto"

	"github.com/cloudfoundry-incubator/garden/drain"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/server/bomberman"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type WardenServer struct {
	listenNetwork string
	listenAddr    string

	containerGraceTime time.Duration
	backend            warden.Backend

	listener     net.Listener
	openRequests *drain.Drain

	setStopping chan bool
	stopping    chan bool

	bomberman *bomberman.Bomberman
}

type UnhandledRequestError struct {
	Request proto.Message
}

func (e UnhandledRequestError) Error() string {
	return fmt.Sprintf("unhandled request type: %T", e.Request)
}

func New(
	listenNetwork, listenAddr string,
	containerGraceTime time.Duration,
	backend warden.Backend,
) *WardenServer {
	return &WardenServer{
		listenNetwork: listenNetwork,
		listenAddr:    listenAddr,

		containerGraceTime: containerGraceTime,
		backend:            backend,

		setStopping: make(chan bool),
		stopping:    make(chan bool),

		openRequests: drain.New(),
	}
}

func (s *WardenServer) Start() error {
	err := s.removeExistingSocket()
	if err != nil {
		return err
	}

	err = s.backend.Start()
	if err != nil {
		return err
	}

	listener, err := net.Listen(s.listenNetwork, s.listenAddr)
	if err != nil {
		return err
	}

	s.listener = listener

	if s.listenNetwork == "unix" {
		os.Chmod(s.listenAddr, 0777)
	}

	containers, err := s.backend.Containers(nil)
	if err != nil {
		return err
	}

	s.bomberman = bomberman.New(s.backend, s.reapContainer)

	for _, container := range containers {
		s.bomberman.Strap(container)
	}

	go s.trackStopping()
	go s.handleConnections(listener)

	return nil
}

func (s *WardenServer) Stop() {
	s.setStopping <- true
	s.listener.Close()
	s.openRequests.Wait()
	s.backend.Stop()
}

func (s *WardenServer) trackStopping() {
	stopping := false

	for {
		select {
		case stopping = <-s.setStopping:
		case s.stopping <- stopping:
		}
	}
}

func (s *WardenServer) handleConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// listener closed
			break
		}

		go s.serveConnection(conn)
	}
}

func (s *WardenServer) serveConnection(conn net.Conn) {
	read := bufio.NewReader(conn)

	for {
		var response proto.Message
		var err error

		if <-s.stopping {
			conn.Close()
			break
		}

		request, err := transport.ReadRequest(read)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Println("error reading request:", err)
			continue
		}

		if <-s.stopping {
			conn.Close()
			break
		}

		s.openRequests.Incr()

		switch req := request.(type) {
		case *protocol.PingRequest:
			response, err = s.handlePing(req)
		case *protocol.EchoRequest:
			response, err = s.handleEcho(req)
		case *protocol.CapacityRequest:
			response, err = s.handleCapacity(req)
		case *protocol.CreateRequest:
			response, err = s.handleCreate(req)
		case *protocol.DestroyRequest:
			response, err = s.handleDestroy(req)
		case *protocol.ListRequest:
			response, err = s.handleList(req)
		case *protocol.StopRequest:
			response, err = s.handleStop(req)
		case *protocol.CopyInRequest:
			response, err = s.handleCopyIn(req)
		case *protocol.CopyOutRequest:
			response, err = s.handleCopyOut(req)
		case *protocol.StreamInRequest:
			response, err = s.handleStreamIn(conn, read, req)
		case *protocol.StreamOutRequest:
			response, err = s.handleStreamOut(conn, req)
		case *protocol.RunRequest:
			s.openRequests.Decr()
			response, err = s.handleRun(conn, req)
			s.openRequests.Incr()
		case *protocol.AttachRequest:
			s.openRequests.Decr()
			response, err = s.handleAttach(conn, req)
			s.openRequests.Incr()
		case *protocol.LimitBandwidthRequest:
			response, err = s.handleLimitBandwidth(req)
		case *protocol.LimitMemoryRequest:
			response, err = s.handleLimitMemory(req)
		case *protocol.LimitDiskRequest:
			response, err = s.handleLimitDisk(req)
		case *protocol.LimitCpuRequest:
			response, err = s.handleLimitCpu(req)
		case *protocol.NetInRequest:
			response, err = s.handleNetIn(req)
		case *protocol.NetOutRequest:
			response, err = s.handleNetOut(req)
		case *protocol.InfoRequest:
			response, err = s.handleInfo(req)
		default:
			err = UnhandledRequestError{request}
		}

		if err != nil {
			response = &protocol.ErrorResponse{
				Message: proto.String(err.Error()),
			}
		}

		if response != nil {
			protocol.Messages(response).WriteTo(conn)
		}

		s.openRequests.Decr()
	}
}

func (s *WardenServer) removeExistingSocket() error {
	if s.listenNetwork != "unix" {
		return nil
	}

	if _, err := os.Stat(s.listenAddr); os.IsNotExist(err) {
		return nil
	}

	err := os.Remove(s.listenAddr)

	if err != nil {
		return fmt.Errorf("error deleting existing socket: %s", err)
	}

	return nil
}

func (s *WardenServer) reapContainer(container warden.Container) {
	log.Printf("reaping %s (idle for %s)\n", container.Handle(), s.backend.GraceTime(container))
	s.backend.Destroy(container.Handle())
}
