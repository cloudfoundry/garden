package fake_connection

import (
	"sync"

	"github.com/cloudfoundry-incubator/garden/warden"
)

type FakeConnection struct {
	lock *sync.RWMutex

	closed bool

	disconnected chan struct{}

	created      []warden.ContainerSpec
	WhenCreating func(spec warden.ContainerSpec) (string, error)

	listedProperties []warden.Properties
	WhenListing      func(props warden.Properties) ([]string, error)

	destroyed      []string
	WhenDestroying func(handle string) error

	stopped      map[string][]StopSpec
	WhenStopping func(handle string, background, kill bool) error

	WhenGettingInfo func(handle string) (warden.ContainerInfo, error)

	copiedIn      map[string][]CopyInSpec
	WhenCopyingIn func(handle string, src, dst string) error

	copiedOut      map[string][]CopyOutSpec
	WhenCopyingOut func(handle string, src, dst, owner string) error

	limitedBandwidth      map[string][]warden.BandwidthLimits
	WhenLimitingBandwidth func(handle string, limits warden.BandwidthLimits) (warden.BandwidthLimits, error)

	limitedCPU      map[string][]warden.CPULimits
	WhenLimitingCPU func(handle string, limits warden.CPULimits) (warden.CPULimits, error)

	limitedDisk      map[string][]warden.DiskLimits
	WhenLimitingDisk func(handle string, limits warden.DiskLimits) (warden.DiskLimits, error)

	limitedMemory      map[string][]warden.MemoryLimits
	WhenLimitingMemory func(handle string, limit warden.MemoryLimits) (warden.MemoryLimits, error)

	spawnedProcesses map[string][]warden.ProcessSpec
	WhenRunning      func(handle string, spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error)

	attachedProcesses map[string][]uint32
	WhenAttaching     func(handle string, processID uint32) (<-chan warden.ProcessStream, error)

	netInned      map[string][]NetInSpec
	WhenNetInning func(handle string, hostPort, containerPort uint32) (uint32, uint32, error)

	netOuted      map[string][]NetOutSpec
	WhenNetOuting func(handle string, network string, port uint32) error
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

func (connection *FakeConnection) Create(spec warden.ContainerSpec) (string, error) {
	connection.lock.Lock()
	connection.created = append(connection.created, spec)
	connection.lock.Unlock()

	if connection.WhenCreating != nil {
		return connection.WhenCreating(spec)
	}

	return "", nil
}

func (connection *FakeConnection) Created() []warden.ContainerSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.created
}

func (connection *FakeConnection) List(properties warden.Properties) ([]string, error) {
	connection.lock.Lock()
	connection.listedProperties = append(connection.listedProperties, properties)
	connection.lock.Unlock()

	if connection.WhenListing != nil {
		return connection.WhenListing(properties)
	}

	return nil, nil
}

func (connection *FakeConnection) Destroy(handle string) error {
	connection.lock.Lock()
	connection.destroyed = append(connection.destroyed, handle)
	connection.lock.Unlock()

	if connection.WhenDestroying != nil {
		return connection.WhenDestroying(handle)
	}

	return nil
}

func (connection *FakeConnection) Destroyed() []string {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.destroyed
}

func (connection *FakeConnection) Stop(handle string, background, kill bool) error {
	connection.lock.Lock()
	connection.stopped[handle] = append(connection.stopped[handle], StopSpec{
		Background: background,
		Kill:       kill,
	})
	connection.lock.Unlock()

	if connection.WhenStopping != nil {
		return connection.WhenStopping(handle, background, kill)
	}

	return nil
}

func (connection *FakeConnection) Stopped(handle string) []StopSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.stopped[handle]
}

func (connection *FakeConnection) Info(handle string) (warden.ContainerInfo, error) {
	if connection.WhenGettingInfo != nil {
		return connection.WhenGettingInfo(handle)
	}

	return warden.ContainerInfo{}, nil
}

func (connection *FakeConnection) CopyIn(handle string, src, dst string) error {
	connection.lock.Lock()
	connection.copiedIn[handle] = append(connection.copiedIn[handle], CopyInSpec{
		Source:      src,
		Destination: dst,
	})
	connection.lock.Unlock()

	if connection.WhenCopyingIn != nil {
		return connection.WhenCopyingIn(handle, src, dst)
	}

	return nil
}

func (connection *FakeConnection) CopiedIn(handle string) []CopyInSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.copiedIn[handle]
}

func (connection *FakeConnection) CopyOut(handle string, src, dst, owner string) error {
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

	return nil
}

func (connection *FakeConnection) CopiedOut(handle string) []CopyOutSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.copiedOut[handle]
}

func (connection *FakeConnection) LimitBandwidth(handle string, limits warden.BandwidthLimits) (warden.BandwidthLimits, error) {
	connection.lock.Lock()
	connection.limitedBandwidth[handle] = append(connection.limitedBandwidth[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingBandwidth != nil {
		return connection.WhenLimitingBandwidth(handle, limits)
	}

	return limits, nil
}

func (connection *FakeConnection) LimitedBandwidth(handle string) []warden.BandwidthLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedBandwidth[handle]
}

func (connection *FakeConnection) LimitCPU(handle string, limits warden.CPULimits) (warden.CPULimits, error) {
	connection.lock.Lock()
	connection.limitedCPU[handle] = append(connection.limitedCPU[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingCPU != nil {
		return connection.WhenLimitingCPU(handle, limits)
	}

	return limits, nil
}

func (connection *FakeConnection) LimitedCPU(handle string) []warden.CPULimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedCPU[handle]
}

func (connection *FakeConnection) LimitDisk(handle string, limits warden.DiskLimits) (warden.DiskLimits, error) {
	connection.lock.Lock()
	connection.limitedDisk[handle] = append(connection.limitedDisk[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingDisk != nil {
		return connection.WhenLimitingDisk(handle, limits)
	}

	return limits, nil
}

func (connection *FakeConnection) LimitedDisk(handle string) []warden.DiskLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedDisk[handle]
}

func (connection *FakeConnection) LimitMemory(handle string, limits warden.MemoryLimits) (warden.MemoryLimits, error) {
	connection.lock.Lock()
	connection.limitedMemory[handle] = append(connection.limitedMemory[handle], limits)
	connection.lock.Unlock()

	if connection.WhenLimitingMemory != nil {
		return connection.WhenLimitingMemory(handle, limits)
	}

	return limits, nil
}

func (connection *FakeConnection) LimitedMemory(handle string) []warden.MemoryLimits {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.limitedMemory[handle]
}

func (connection *FakeConnection) Run(handle string, spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
	connection.lock.Lock()
	connection.spawnedProcesses[handle] = append(connection.spawnedProcesses[handle], spec)
	connection.lock.Unlock()

	if connection.WhenRunning != nil {
		return connection.WhenRunning(handle, spec)
	}

	return 0, make(chan warden.ProcessStream), nil
}

func (connection *FakeConnection) SpawnedProcesses(handle string) []warden.ProcessSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.spawnedProcesses[handle]
}

func (connection *FakeConnection) Attach(handle string, processID uint32) (<-chan warden.ProcessStream, error) {
	connection.lock.Lock()
	connection.attachedProcesses[handle] = append(connection.attachedProcesses[handle], processID)
	connection.lock.Unlock()

	if connection.WhenAttaching != nil {
		return connection.WhenAttaching(handle, processID)
	}

	return make(chan warden.ProcessStream), nil
}

func (connection *FakeConnection) AttachedProcesses(handle string) []uint32 {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.attachedProcesses[handle]
}

func (connection *FakeConnection) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	connection.lock.Lock()
	connection.netInned[handle] = append(connection.netInned[handle], NetInSpec{
		HostPort:      hostPort,
		ContainerPort: containerPort,
	})
	connection.lock.Unlock()

	if connection.WhenNetInning != nil {
		return connection.WhenNetInning(handle, hostPort, containerPort)
	}

	return 0, 0, nil
}

func (connection *FakeConnection) NetInned(handle string) []NetInSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.netInned[handle]
}

func (connection *FakeConnection) NetOut(handle string, network string, port uint32) error {
	connection.lock.Lock()
	connection.netOuted[handle] = append(connection.netOuted[handle], NetOutSpec{
		Network: network,
		Port:    port,
	})
	connection.lock.Unlock()

	if connection.WhenNetOuting != nil {
		return connection.WhenNetOuting(handle, network, port)
	}

	return nil
}

func (connection *FakeConnection) NetOuted(handle string) []NetOutSpec {
	connection.lock.RLock()
	defer connection.lock.RUnlock()

	return connection.netOuted[handle]
}
