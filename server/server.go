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
	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/cloudfoundry-incubator/garden/server/bomberman"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/tedsuo/rata"
)

type WardenServer struct {
	server        http.Server
	listenNetwork string
	listenAddr    string

	containerGraceTime time.Duration
	backend            warden.Backend

	listener net.Listener
	handling *sync.WaitGroup

	stopping chan bool

	bomberman *bomberman.Bomberman

	conns map[net.Conn]net.Conn
	mu    sync.Mutex
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
	s := &WardenServer{
		listenNetwork: listenNetwork,
		listenAddr:    listenAddr,

		containerGraceTime: containerGraceTime,
		backend:            backend,

		stopping: make(chan bool),

		handling: new(sync.WaitGroup),
		conns:    make(map[net.Conn]net.Conn),
	}

	handlers := map[string]http.Handler{
		routes.Ping:                   http.HandlerFunc(s.handlePing),
		routes.Capacity:               http.HandlerFunc(s.handleCapacity),
		routes.Create:                 http.HandlerFunc(s.handleCreate),
		routes.Destroy:                http.HandlerFunc(s.handleDestroy),
		routes.List:                   http.HandlerFunc(s.handleList),
		routes.Stop:                   http.HandlerFunc(s.handleStop),
		routes.StreamIn:               http.HandlerFunc(s.handleStreamIn),
		routes.StreamOut:              http.HandlerFunc(s.handleStreamOut),
		routes.LimitBandwidth:         http.HandlerFunc(s.handleLimitBandwidth),
		routes.CurrentBandwidthLimits: http.HandlerFunc(s.handleCurrentBandwidthLimits),
		routes.LimitCPU:               http.HandlerFunc(s.handleLimitCPU),
		routes.CurrentCPULimits:       http.HandlerFunc(s.handleCurrentCPULimits),
		routes.LimitDisk:              http.HandlerFunc(s.handleLimitDisk),
		routes.CurrentDiskLimits:      http.HandlerFunc(s.handleCurrentDiskLimits),
		routes.LimitMemory:            http.HandlerFunc(s.handleLimitMemory),
		routes.CurrentMemoryLimits:    http.HandlerFunc(s.handleCurrentMemoryLimits),
		routes.NetIn:                  http.HandlerFunc(s.handleNetIn),
		routes.NetOut:                 http.HandlerFunc(s.handleNetOut),
		routes.Info:                   http.HandlerFunc(s.handleInfo),
		routes.Run:                    http.HandlerFunc(s.handleRun),
		routes.Attach:                 http.HandlerFunc(s.handleAttach),
	}

	mux, err := rata.NewRouter(routes.Routes, handlers)
	if err != nil {
		log.Fatalln("failed to initialize router:", err)
	}

	s.server = http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r)
		}),

		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				s.handling.Add(1)
			case http.StateActive:
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
			case http.StateIdle:
				select {
				case <-s.stopping:
					conn.Close()
				default:
					s.mu.Lock()
					s.conns[conn] = conn
					s.mu.Unlock()
				}
			case http.StateHijacked, http.StateClosed:
				s.mu.Lock()
				delete(s.conns, conn)
				s.mu.Unlock()
				s.handling.Done()
			}
		},
	}

	return s
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

	go s.server.Serve(listener)

	return nil
}

func (s *WardenServer) Stop() {
	close(s.stopping)

	s.listener.Close()

	s.mu.Lock()
	conns := s.conns
	s.conns = make(map[net.Conn]net.Conn)
	s.mu.Unlock()

	for _, c := range conns {
		log.Println("closing idle connection:", c.RemoteAddr())
		c.Close()
	}

	log.Println("waiting for active connections to complete")
	s.handling.Wait()

	log.Println("waiting for backend to stop")
	s.backend.Stop()

	log.Println("garden server stopped")
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
