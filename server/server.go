package server

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"code.google.com/p/gogoprotobuf/proto"
	"github.com/cloudfoundry-incubator/garden/server/bomberman"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type WardenServer struct {
	listenNetwork string
	listenAddr    string

	containerGraceTime time.Duration
	backend            warden.Backend

	listener net.Listener
	handling *sync.WaitGroup

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

		handling: new(sync.WaitGroup),
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
	s.handling.Wait()
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
	mux := http.NewServeMux()

	mux.HandleFunc("/ping", s.handlePing)
	mux.HandleFunc("/capacity", s.handleCapacity)
	mux.HandleFunc("/create", s.handleCreate)
	mux.HandleFunc("/destroy", s.handleDestroy)
	mux.HandleFunc("/list", s.handleList)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/stream_in", s.handleStreamIn)
	mux.HandleFunc("/stream_out", s.handleStreamOut)
	mux.HandleFunc("/limit_bandwidth", s.handleLimitBandwidth)
	mux.HandleFunc("/limit_memory", s.handleLimitMemory)
	mux.HandleFunc("/limit_disk", s.handleLimitDisk)
	mux.HandleFunc("/limit_cpu", s.handleLimitCpu)
	mux.HandleFunc("/net_in", s.handleNetIn)
	mux.HandleFunc("/net_out", s.handleNetOut)
	mux.HandleFunc("/info", s.handleInfo)
	mux.HandleFunc("/run", s.handleRun)
	mux.HandleFunc("/attach", s.handleAttach)

	server := http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r)
			s.handling.Done()
		}),

		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew {
				return
			}

			if state == http.StateActive {
				s.handling.Add(1)

				if <-s.stopping {
					conn.Close()
				}
			}
		},
	}

	server.Serve(listener)
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
