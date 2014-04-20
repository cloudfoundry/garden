package fake_connection

import (
	"fmt"
	"sync"

	"code.google.com/p/goprotobuf/proto"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type FakeConnection struct {
	lock *sync.RWMutex

	closed bool

	disconnected chan struct{}

	created      []warden.ContainerSpec
	WhenCreating func(spec warden.ContainerSpec) (*protocol.CreateResponse, error)

	listedProperties []warden.Properties
	WhenListing      func(props warden.Properties) (*protocol.ListResponse, error)

	destroyed      []string
	WhenDestroying func(handle string) (*protocol.DestroyResponse, error)

	stopped      map[string][]StopSpec
	WhenStopping func(handle string, background, kill bool) (*protocol.StopResponse, error)

	WhenGettingInfo func(handle string) (*protocol.InfoResponse, error)

	copiedIn      map[string][]CopyInSpec
	WhenCopyingIn func(handle string, src, dst string) (*protocol.CopyInResponse, error)

	copiedOut      map[string][]CopyOutSpec
	WhenCopyingOut func(handle string, src, dst, owner string) (*protocol.CopyOutResponse, error)

	limitedBandwidth      map[string][]warden.BandwidthLimits
	WhenLimitingBandwidth func(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error)

	limitedCPU      map[string][]warden.CPULimits
	WhenLimitingCPU func(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error)

	limitedDisk      map[string][]warden.DiskLimits
	WhenLimitingDisk func(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error)

	limitedMemory      map[string][]warden.MemoryLimits
	WhenLimitingMemory func(handle string, limit warden.MemoryLimits) (*protocol.LimitMemoryResponse, error)

	spawnedProcesses map[string][]warden.ProcessSpec
	WhenRunning      func(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error)

	attachedProcesses map[string][]uint32
	WhenAttaching     func(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error)

	netInned      map[string][]NetInSpec
	WhenNetInning func(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error)

	netOuted      map[string][]NetOutSpec
	WhenNetOuting func(handle string, network string, port uint32) (*protocol.NetOutResponse, error)
}

type StopSpec struct {
	Background bool
	Kill       bool
}

type CopyInSpec struct {
	Source      string
	Destination string
}

type CopyOutSpec struct {
	Source      string
	Destination string
	Owner       string
}

type NetInSpec struct {
	HostPort      uint32
	ContainerPort uint32
}

type NetOutSpec struct {
	Network string
	Port    uint32
}

func New() *FakeConnection {
	return &FakeConnection{
		lock: &sync.RWMutex{},

		disconnected: make(chan struct{}),

		stopped: make(map[string][]StopSpec),

		copiedIn:  make(map[string][]CopyInSpec),
		copiedOut: make(map[string][]CopyOutSpec),

		limitedBandwidth: make(map[string][]warden.BandwidthLimits),
		limitedCPU:       make(map[string][]warden.CPULimits),
		limitedDisk:      make(map[string][]warden.DiskLimits),
		limitedMemory:    make(map[string][]warden.MemoryLimits),

		spawnedProcesses:  make(map[string][]warden.ProcessSpec),
		attachedProcesses: make(map[string][]uint32),

		netInned: make(map[string][]NetInSpec),
		netOuted: make(map[string][]NetOutSpec),
	}
}

func (connection *FakeConnection) Close() {
	connection.lock.Lock()
	connection.closed = true
	connection.lock.Unlock()
}

func (connection *FakeConnection) IsClosed() bool {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.closed
}

func (connection *FakeConnection) Disconnected() <-chan struct{} {
	return connection.disconnected
}

func (connection *FakeConnection) NotifyDisconnected() {
	close(connection.disconnected)
}

func (connection *FakeConnection) RoundTrip(request proto.Message, response proto.Message) (proto.Message, error) {
	panic("implement me!")
	return nil, nil
}

func (connection *FakeConnection) Create(spec warden.ContainerSpec) (*protocol.CreateResponse, error) {
	connection.lock.Lock()
	connection.created = append(connection.created, spec)
	handle := fmt.Sprintf("handle-%d", len(connection.created))
	connection.lock.Unlock()

	if connection.WhenCreating != nil {
		return connection.WhenCreating(spec)
	}

	return &protocol.CreateResponse{Handle: proto.String(handle)}, nil
}

func (connection *FakeConnection) Created() []warden.ContainerSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.created
}

func (connection *FakeConnection) List(properties warden.Properties) (*protocol.ListResponse, error) {
	connection.lock.Lock()
	connection.listedProperties = append(connection.listedProperties, properties)
	connection.lock.Unlock()

	if connection.WhenListing != nil {
		return connection.WhenListing(properties)
	}

	return &protocol.ListResponse{}, nil
}

func (connection *FakeConnection) Destroy(handle string) (*protocol.DestroyResponse, error) {
	connection.lock.Lock()
	connection.destroyed = append(connection.destroyed, handle)
	connection.lock.Unlock()

	if connection.WhenDestroying != nil {
		return connection.WhenDestroying(handle)
	}

	return &protocol.DestroyResponse{}, nil
}

func (connection *FakeConnection) Destroyed() []string {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.destroyed
}

func (connection *FakeConnection) Stop(handle string, background, kill bool) (*protocol.StopResponse, error) {
	connection.lock.Lock()
	connection.stopped[handle] = append(connection.stopped[handle], StopSpec{
		Background: background,
		Kill:       kill,
	})
	connection.lock.Unlock()

	if connection.WhenStopping != nil {
		return connection.WhenStopping(handle, background, kill)
	}

	return &protocol.StopResponse{}, nil
}

func (connection *FakeConnection) Stopped(handle string) []StopSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.stopped[handle]
}

func (connection *FakeConnection) Info(handle string) (*protocol.InfoResponse, error) {
	if connection.WhenGettingInfo != nil {
		return connection.WhenGettingInfo(handle)
	}

	return &protocol.InfoResponse{}, nil
}

func (connection *FakeConnection) CopyIn(handle string, src, dst string) (*protocol.CopyInResponse, error) {
	connection.lock.Lock()
	connection.copiedIn[handle] = append(connection.copiedIn[handle], CopyInSpec{
		Source:      src,
		Destination: dst,
	})
	connection.lock.Unlock()

	if connection.WhenCopyingIn != nil {
		return connection.WhenCopyingIn(handle, src, dst)
	}

	return &protocol.CopyInResponse{}, nil
}

func (connection *FakeConnection) CopiedIn(handle string) []CopyInSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.copiedIn[handle]
}

func (connection *FakeConnection) CopyOut(handle string, src, dst, owner string) (*protocol.CopyOutResponse, error) {
	connection.lock.Lock()
	connection.copiedOut[handle] = append(connection.copiedOut[handle], CopyOutSpec{
		Source:      src,
		Destination: dst,
		Owner:       owner,
	})
	connection.lock.Unlock()

	if connection.WhenCopyingOut != nil {
		return connection.WhenCopyingOut(handle, src, dst, owner)
	}

	return &protocol.CopyOutResponse{}, nil
}

func (connection *FakeConnection) CopiedOut(handle string) []CopyOutSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.copiedOut[handle]
}

func (connection *FakeConnection) LimitBandwidth(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error) {
	connection.lock.Lock()
	connection.limitedBandwidth[handle] = append(connection.limitedBandwidth[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingBandwidth != nil {
		return connection.WhenLimitingBandwidth(handle, limits)
	}

	return &protocol.LimitBandwidthResponse{}, nil
}

func (connection *FakeConnection) LimitedBandwidth(handle string) []warden.BandwidthLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedBandwidth[handle]
}

func (connection *FakeConnection) LimitCPU(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error) {
	connection.lock.Lock()
	connection.limitedCPU[handle] = append(connection.limitedCPU[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingCPU != nil {
		return connection.WhenLimitingCPU(handle, limits)
	}

	return &protocol.LimitCpuResponse{}, nil
}

func (connection *FakeConnection) LimitedCPU(handle string) []warden.CPULimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedCPU[handle]
}

func (connection *FakeConnection) LimitDisk(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error) {
	connection.lock.Lock()
	connection.limitedDisk[handle] = append(connection.limitedDisk[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingDisk != nil {
		return connection.WhenLimitingDisk(handle, limits)
	}

	return &protocol.LimitDiskResponse{}, nil
}

func (connection *FakeConnection) LimitedDisk(handle string) []warden.DiskLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedDisk[handle]
}

func (connection *FakeConnection) LimitMemory(handle string, limits warden.MemoryLimits) (*protocol.LimitMemoryResponse, error) {
	connection.lock.Lock()
	connection.limitedMemory[handle] = append(connection.limitedMemory[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingMemory != nil {
		return connection.WhenLimitingMemory(handle, limits)
	}

	return &protocol.LimitMemoryResponse{}, nil
}

func (connection *FakeConnection) LimitedMemory(handle string) []warden.MemoryLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedMemory[handle]
}

func (connection *FakeConnection) Run(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error) {
	connection.lock.Lock()
	connection.spawnedProcesses[handle] = append(connection.spawnedProcesses[handle], spec)
	connection.lock.Unlock()

	if connection.WhenRunning != nil {
		return connection.WhenRunning(handle, spec)
	}

	return &protocol.ProcessPayload{}, make(chan *protocol.ProcessPayload), nil
}

func (connection *FakeConnection) SpawnedProcesses(handle string) []warden.ProcessSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.spawnedProcesses[handle]
}

func (connection *FakeConnection) Attach(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error) {
	connection.lock.Lock()
	connection.attachedProcesses[handle] = append(connection.attachedProcesses[handle], processID)
	connection.lock.Unlock()

	if connection.WhenAttaching != nil {
		return connection.WhenAttaching(handle, processID)
	}

	return make(chan *protocol.ProcessPayload), nil
}

func (connection *FakeConnection) AttachedProcesses(handle string) []uint32 {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.attachedProcesses[handle]
}

func (connection *FakeConnection) NetIn(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error) {
	connection.lock.Lock()
	connection.netInned[handle] = append(connection.netInned[handle], NetInSpec{
		HostPort:      hostPort,
		ContainerPort: containerPort,
	})
	connection.lock.Unlock()

	if connection.WhenNetInning != nil {
		return connection.WhenNetInning(handle, hostPort, containerPort)
	}

	return &protocol.NetInResponse{}, nil
}

func (connection *FakeConnection) NetInned(handle string) []NetInSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.netInned[handle]
}

func (connection *FakeConnection) NetOut(handle string, network string, port uint32) (*protocol.NetOutResponse, error) {
	connection.lock.Lock()
	connection.netOuted[handle] = append(connection.netOuted[handle], NetOutSpec{
		Network: network,
		Port:    port,
	})
	connection.lock.Unlock()

	if connection.WhenNetOuting != nil {
		return connection.WhenNetOuting(handle, network, port)
	}

	return &protocol.NetOutResponse{}, nil
}

func (connection *FakeConnection) NetOuted(handle string) []NetOutSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.netOuted[handle]
}
