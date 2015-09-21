package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/routes"
	"github.com/cloudfoundry-incubator/garden/server/bomberman"
	"github.com/cloudfoundry-incubator/garden/server/streamer"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

type GardenServer struct {
	logger lager.Logger

	server        http.Server
	listenNetwork string
	listenAddr    string

	containerGraceTime time.Duration
	backend            garden.Backend

	listener net.Listener
	handling *sync.WaitGroup

	started  bool
	stopping chan bool

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

	s.server = http.Server{
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

func (s *GardenServer) Start() error {
	s.started = true

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

func (s *GardenServer) Stop() {
	if !s.started {
		return
	}

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
	s.backend.Stop()

	s.logger.Info("stopped")
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

	s.backend.Destroy(container.Handle())
}
