package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/routes"
	"code.cloudfoundry.org/garden/server/bomberman"
	"code.cloudfoundry.org/garden/server/streamer"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"
)

type GardenServer struct {
	logger lager.Logger

	server        *http.Server
	listenNetwork string
	listenAddr    string

	containerGraceTime time.Duration
	backend            garden.Backend

	listener net.Listener
	handling *sync.WaitGroup

	started    bool
	startMutex *sync.Mutex
	stopping   chan bool

	bomberman *bomberman.Bomberman

	conns map[net.Conn]net.Conn
	mu    sync.Mutex

	streamer *streamer.Streamer

	destroys  map[string]struct{}
	destroysL *sync.Mutex
}

func New(
	listenNetwork, listenAddr string,
	containerGraceTime time.Duration,
	backend garden.Backend,
	logger lager.Logger,
) *GardenServer {
	s := &GardenServer{
		logger: logger.Session("garden-server"),

		listenNetwork: listenNetwork,
		listenAddr:    listenAddr,

		containerGraceTime: containerGraceTime,
		backend:            backend,

		stopping: make(chan bool),

		handling: new(sync.WaitGroup),
		conns:    make(map[net.Conn]net.Conn),

		streamer: streamer.New(time.Minute),

		destroys:  make(map[string]struct{}),
		destroysL: new(sync.Mutex),

		startMutex: new(sync.Mutex),
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
		routes.CurrentBandwidthLimits: http.HandlerFunc(s.handleCurrentBandwidthLimits),
		routes.CurrentCPULimits:       http.HandlerFunc(s.handleCurrentCPULimits),
		routes.CurrentDiskLimits:      http.HandlerFunc(s.handleCurrentDiskLimits),
		routes.CurrentMemoryLimits:    http.HandlerFunc(s.handleCurrentMemoryLimits),
		routes.NetIn:                  http.HandlerFunc(s.handleNetIn),
		routes.NetOut:                 http.HandlerFunc(s.handleNetOut),
		routes.BulkNetOut:             http.HandlerFunc(s.handleBulkNetOut),
		routes.Info:                   http.HandlerFunc(s.handleInfo),
		routes.BulkInfo:               http.HandlerFunc(s.handleBulkInfo),
		routes.BulkMetrics:            http.HandlerFunc(s.handleBulkMetrics),
		routes.Run:                    http.HandlerFunc(s.handleRun),
		routes.Stdout:                 streamer.HandlerFunc(s.streamer.ServeStdout),
		routes.Stderr:                 streamer.HandlerFunc(s.streamer.ServeStderr),
		routes.Attach:                 http.HandlerFunc(s.handleAttach),
		routes.Metrics:                http.HandlerFunc(s.handleMetrics),
		routes.Properties:             http.HandlerFunc(s.handleProperties),
		routes.Property:               http.HandlerFunc(s.handleProperty),
		routes.SetProperty:            http.HandlerFunc(s.handleSetProperty),
		routes.RemoveProperty:         http.HandlerFunc(s.handleRemoveProperty),
		routes.SetGraceTime:           http.HandlerFunc(s.handleSetGraceTime),
	}

	mux, err := rata.NewRouter(routes.Routes, handlers)
	if err != nil {
		logger.Fatal("failed-to-initialize-rata", err)
	}

	conLogger := logger.Session("connection")

	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r)
		}),

		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				conLogger.Debug("open", lager.Data{"local_addr": conn.LocalAddr(), "remote_addr": conn.RemoteAddr()})
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
				conLogger.Debug("closed", lager.Data{"local_addr": conn.LocalAddr(), "remote_addr": conn.RemoteAddr()})
				s.handling.Done()
			}
		},
	}

	return s
}

func (s *GardenServer) ListenAndServe() error {
	listener, err := s.listen()
	if err != nil {
		return err
	}

	return s.Serve(listener)
}

func (s *GardenServer) Serve(listener net.Listener) error {
	s.startMutex.Lock()
	s.started = true
	s.listener = listener
	s.startMutex.Unlock()

	err := s.server.Serve(s.listener)
	s.startMutex.Lock()
	defer s.startMutex.Unlock()
	if s.started {
		return err
	}
	return nil
}

// Start deprecated: please use Serve() or ListenAndServe()
func (s *GardenServer) Start() error {
	if err := s.backend.Start(); err != nil {
		return err
	}

	if err := s.SetupBomberman(); err != nil {
		return err
	}

	listener, err := s.listen()
	if err != nil {
		return err
	}

	go s.server.Serve(listener)

	return nil
}

func (s *GardenServer) SetupBomberman() error {
	containers, err := s.backend.Containers(nil)
	if err != nil {
		return err
	}

	s.bomberman = bomberman.New(s.backend, s.reapContainer)

	for _, container := range containers {
		s.bomberman.Strap(container)
	}

	return nil
}

func (s *GardenServer) listen() (net.Listener, error) {
	if err := s.removeExistingSocket(); err != nil {
		return nil, err
	}

	listener, err := net.Listen(s.listenNetwork, s.listenAddr)
	if err != nil {
		return nil, err
	}

	if s.listenNetwork == "unix" {
		// The permissions of this socket are not ideal, and have been globally
		// readable/writeable for years due the fact that in Cloud Foundry
		// deployments, garden server and diego rep always run as different users.
		// https://www.pivotaltracker.com/story/show/151245015 addresses this
		// issue.
		if err := os.Chmod(s.listenAddr, 0777); err != nil {
			return nil, err
		}
	}

	return listener, nil
}

func (s *GardenServer) Stop() error {
	s.startMutex.Lock()
	defer s.startMutex.Unlock()
	if !s.started {
		return errors.New("server is not running")
	}
	s.started = false

	close(s.stopping)

	s.listener.Close()

	s.mu.Lock()
	conns := s.conns
	s.conns = make(map[net.Conn]net.Conn)
	s.mu.Unlock()

	for _, c := range conns {
		s.logger.Debug("closing-idle", lager.Data{
			"addr": c.RemoteAddr(),
		})

		c.Close()
	}

	s.logger.Info("waiting-for-connections-to-close")
	s.handling.Wait()

	s.logger.Info("stopping-backend")
	err := s.backend.Stop()
	if err != nil {
		return err
	}

	s.logger.Info("stopped")
	return nil
}

func (s *GardenServer) removeExistingSocket() error {
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

func (s *GardenServer) reapContainer(container garden.Container) {
	s.logger.Info("reaping", lager.Data{
		"handle":     container.Handle(),
		"grace-time": s.backend.GraceTime(container).String(),
	})

	s.destroysL.Lock()
	_, alreadyDestroying := s.destroys[container.Handle()]
	if !alreadyDestroying {
		s.destroys[container.Handle()] = struct{}{}
	}
	s.destroysL.Unlock()

	if alreadyDestroying {
		s.logger.Info("skipping reap due to concurrent delete request", lager.Data{
			"handle":     container.Handle(),
			"grace-time": s.backend.GraceTime(container).String(),
		})
		return
	}

	s.backend.Destroy(container.Handle())

	s.destroysL.Lock()
	delete(s.destroys, container.Handle())
	s.destroysL.Unlock()
}
