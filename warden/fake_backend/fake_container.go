package fake_backend

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/nu7hatch/gouuid"
	"github.com/onsi/gomega/gbytes"

	"github.com/cloudfoundry-incubator/garden/warden"
)

type FakeContainer struct {
	id string

	Spec warden.ContainerSpec

	StartError error
	Started    bool

	StopError    error
	stopped      []StopSpec
	stopMutex    *sync.RWMutex
	StopCallback func()

	CleanedUp bool

	StreamInError error
	StreamedIn    []StreamInSpec

	StreamOutError  error
	StreamOutBuffer *bytes.Buffer
	StreamedOut     []string

	RunError         error
	RunningProcessID uint32
	RunningProcesses []warden.ProcessSpec

	AttachError error
	Attached    []uint32

	StreamedProcessChunks []warden.ProcessStream
	StreamDelay           time.Duration

	DidLimitBandwidth   bool
	LimitBandwidthError error
	LimitedBandwidth    warden.BandwidthLimits

	CurrentBandwidthLimitsResult warden.BandwidthLimits
	CurrentBandwidthLimitsError  error

	DidLimitMemory   bool
	LimitMemoryError error
	LimitedMemory    warden.MemoryLimits

	CurrentMemoryLimitsResult warden.MemoryLimits
	CurrentMemoryLimitsError  error

	DidLimitDisk   bool
	LimitDiskError error
	LimitedDisk    warden.DiskLimits

	CurrentDiskLimitsResult warden.DiskLimits
	CurrentDiskLimitsError  error

	DidLimitCPU   bool
	LimitCPUError error
	LimitedCPU    warden.CPULimits

	CurrentCPULimitsResult warden.CPULimits
	CurrentCPULimitsError  error

	NetInError error
	MappedIn   [][]uint32

	NetOutError  error
	PermittedOut []NetOutSpec

	InfoError    error
	ReportedInfo warden.ContainerInfo

	SnapshotError  error
	SavedSnapshots []io.Writer
	snapshotMutex  *sync.RWMutex
}

type NetOutSpec struct {
	Network string
	Port    uint32
}

type StopSpec struct {
	Killed bool
}

type StreamInSpec struct {
	InStream     *gbytes.Buffer
	DestPath     string
	CloseTracker *CloseTracker
}

func NewFakeContainer(spec warden.ContainerSpec) *FakeContainer {
	idUUID, err := uuid.NewV4()
	if err != nil {
		panic("could not create uuid: " + err.Error())
	}

	id := idUUID.String()[:11]

	if spec.Handle == "" {
		spec.Handle = id
	}

	return &FakeContainer{
		id: id,

		Spec: spec,

		StreamOutBuffer: new(bytes.Buffer),

		stopMutex:     new(sync.RWMutex),
		snapshotMutex: new(sync.RWMutex),
	}
}

func (c *FakeContainer) ID() string {
	return c.Spec.Handle
}

func (c *FakeContainer) Handle() string {
	return c.Spec.Handle
}

func (c *FakeContainer) Properties() warden.Properties {
	return c.Spec.Properties
}

func (c *FakeContainer) GraceTime() time.Duration {
	return c.Spec.GraceTime
}

func (c *FakeContainer) Snapshot(snapshot io.Writer) error {
	if c.SnapshotError != nil {
		return c.SnapshotError
	}

	c.snapshotMutex.Lock()
	defer c.snapshotMutex.Unlock()

	c.SavedSnapshots = append(c.SavedSnapshots, snapshot)

	return nil
}

func (c *FakeContainer) Start() error {
	if c.StartError != nil {
		return c.StartError
	}

	c.Started = true

	return nil
}

func (c *FakeContainer) Stop(kill bool) error {
	if c.StopError != nil {
		return c.StopError
	}

	if c.StopCallback != nil {
		c.StopCallback()
	}

	// stops can happen asynchronously in tests (i.e. StopRequest with
	// Background: true), so we need a mutex here
	c.stopMutex.Lock()
	c.stopped = append(c.stopped, StopSpec{kill})
	c.stopMutex.Unlock()

	return nil
}

func (c *FakeContainer) Stopped() []StopSpec {
	c.stopMutex.RLock()
	defer c.stopMutex.RUnlock()

	stopped := make([]StopSpec, len(c.stopped))
	copy(stopped, c.stopped)

	return stopped
}

func (c *FakeContainer) Info() (warden.ContainerInfo, error) {
	if c.InfoError != nil {
		return warden.ContainerInfo{}, c.InfoError
	}

	return c.ReportedInfo, nil
}

func (c *FakeContainer) StreamIn(dst string) (io.WriteCloser, error) {
	buffer := gbytes.NewBuffer()

	closeTracker := NewCloseTracker(nil, buffer)

	c.StreamedIn = append(c.StreamedIn, StreamInSpec{
		InStream:     buffer,
		DestPath:     dst,
		CloseTracker: closeTracker,
	})

	return closeTracker, c.StreamInError
}

func (c *FakeContainer) StreamOut(srcPath string) (io.Reader, error) {
	c.StreamedOut = append(c.StreamedOut, srcPath)
	return c.StreamOutBuffer, c.StreamOutError
}

func (c *FakeContainer) LimitBandwidth(limits warden.BandwidthLimits) error {
	c.DidLimitBandwidth = true

	if c.LimitBandwidthError != nil {
		return c.LimitBandwidthError
	}

	c.LimitedBandwidth = limits

	return nil
}

func (c *FakeContainer) CurrentBandwidthLimits() (warden.BandwidthLimits, error) {
	if c.CurrentBandwidthLimitsError != nil {
		return warden.BandwidthLimits{}, c.CurrentBandwidthLimitsError
	}

	return c.CurrentBandwidthLimitsResult, nil
}

func (c *FakeContainer) LimitDisk(limits warden.DiskLimits) error {
	c.DidLimitDisk = true

	if c.LimitDiskError != nil {
		return c.LimitDiskError
	}

	c.LimitedDisk = limits

	return nil
}

func (c *FakeContainer) CurrentDiskLimits() (warden.DiskLimits, error) {
	if c.CurrentDiskLimitsError != nil {
		return warden.DiskLimits{}, c.CurrentDiskLimitsError
	}

	return c.CurrentDiskLimitsResult, nil
}

func (c *FakeContainer) LimitMemory(limits warden.MemoryLimits) error {
	c.DidLimitMemory = true

	if c.LimitMemoryError != nil {
		return c.LimitMemoryError
	}

	c.LimitedMemory = limits

	return nil
}

func (c *FakeContainer) CurrentMemoryLimits() (warden.MemoryLimits, error) {
	if c.CurrentMemoryLimitsError != nil {
		return warden.MemoryLimits{}, c.CurrentMemoryLimitsError
	}

	return c.CurrentMemoryLimitsResult, nil
}

func (c *FakeContainer) LimitCPU(limits warden.CPULimits) error {
	c.DidLimitCPU = true

	if c.LimitCPUError != nil {
		return c.LimitCPUError
	}

	c.LimitedCPU = limits

	return nil
}

func (c *FakeContainer) CurrentCPULimits() (warden.CPULimits, error) {
	if c.CurrentCPULimitsError != nil {
		return warden.CPULimits{}, c.CurrentCPULimitsError
	}

	return c.CurrentCPULimitsResult, nil
}

func (c *FakeContainer) Run(spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
	if c.RunError != nil {
		return 0, nil, c.RunError
	}

	c.RunningProcesses = append(c.RunningProcesses, spec)

	return c.RunningProcessID, c.fakeAttach(), nil
}

func (c *FakeContainer) Attach(processID uint32) (<-chan warden.ProcessStream, error) {
	if c.AttachError != nil {
		return nil, c.AttachError
	}

	c.Attached = append(c.Attached, processID)

	return c.fakeAttach(), nil
}

func (c *FakeContainer) NetIn(hostPort uint32, containerPort uint32) (uint32, uint32, error) {
	if c.NetInError != nil {
		return 0, 0, c.NetInError
	}

	c.MappedIn = append(c.MappedIn, []uint32{hostPort, containerPort})

	return hostPort, containerPort, nil
}

func (c *FakeContainer) NetOut(network string, port uint32) error {
	if c.NetOutError != nil {
		return c.NetOutError
	}

	c.PermittedOut = append(c.PermittedOut, NetOutSpec{network, port})

	return nil
}

func (c *FakeContainer) Cleanup() {
	c.CleanedUp = true
}

func (c *FakeContainer) fakeAttach() chan warden.ProcessStream {
	stream := make(chan warden.ProcessStream, len(c.StreamedProcessChunks))

	go func() {
		for _, chunk := range c.StreamedProcessChunks {
			time.Sleep(c.StreamDelay)
			stream <- chunk
		}

		close(stream)
	}()

	return stream
}
