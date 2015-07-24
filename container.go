package garden

import (
	"io"
)

//go:generate counterfeiter . Container

type Container interface {
	Handle() string

	// Stop stops a container.
	//
	// If kill is false, garden stops a container by sending the processes running inside it the SIGTERM signal.
	// It then waits for the processes to terminate before returning a response.
	// If one or more processes do not terminate within 10 seconds,
	// garden sends these processes the SIGKILL signal, killing them ungracefully.
	//
	// If kill is true, garden stops a container by sending the processing running inside it a SIGKILL signal.
	//
	// Once a container is stopped, garden does not allow spawning new processes inside the container.
	// It is possible to copy files in to and out of a stopped container.
	// It is only when a container is destroyed that its filesystem is cleaned up.
	//
	// Errors:
	// * None.
	Stop(kill bool) error

	// Returns information about a container.
	Info() (ContainerInfo, error)

	// StreamIn streams data into a file in a container.
	//
	// Errors:
	// *  TODO.
	StreamIn(spec StreamInSpec) error

	// StreamOut streams a file out of a container.
	//
	// Errors:
	// * TODO.
	StreamOut(spec StreamOutSpec) (io.ReadCloser, error)

	// Limits the network bandwidth for a container.
	LimitBandwidth(limits BandwidthLimits) error

	CurrentBandwidthLimits() (BandwidthLimits, error)

	// Limits the CPU shares for a container.
	LimitCPU(limits CPULimits) error

	CurrentCPULimits() (CPULimits, error)

	// Limits the disk usage for a container.
	//
	// The disk limits that are set by this command only have effect for the container's unprivileged user.
	// Files/directories created by its privileged user are not subject to these limits.
	//
	// TODO: explain how disk management works.
	LimitDisk(limits DiskLimits) error

	CurrentDiskLimits() (DiskLimits, error)

	// Limits the memory usage for a container.
	//
	// The limit applies to all process in the container. When the limit is
	// exceeded, the container will be automatically stopped.
	//
	// Errors:
	// * The kernel does not support setting memory.memsw.limit_in_bytes.
	LimitMemory(limits MemoryLimits) error

	CurrentMemoryLimits() (MemoryLimits, error)

	// Map a port on the host to a port in the container so that traffic to the
	// host port is forwarded to the container port.
	//
	// If a host port is not given, a port will be acquired from the server's port
	// pool.
	//
	// If a container port is not given, the port will be the same as the
	// container port.
	//
	// The two resulting ports are returned in the response.
	//
	// Errors:
	// * When no port can be acquired from the server's port pool.
	NetIn(hostPort, containerPort uint32) (uint32, uint32, error)

	// Whitelist outbound network traffic.
	//
	// If the configuration directive deny_networks is not used,
	// all networks are already whitelisted and this command is effectively a no-op.
	//
	// Later NetOut calls take precedence over earlier calls, which is
	// significant only in relation to logging.
	//
	// Errors:
	// * An error is returned if the NetOut call fails.
	NetOut(netOutRule NetOutRule) error

	// Run a script inside a container.
	//
	// The root user will be mapped to a non-root UID in the host unless the container (not this process) was created with 'privileged' true.
	//
	// Errors:
	// * TODO.
	Run(ProcessSpec, ProcessIO) (Process, error)

	// Attach starts streaming the output back to the client from a specified process.
	//
	// Errors:
	// * processID does not refer to a running process.
	Attach(processID uint32, io ProcessIO) (Process, error)

	// Metrics returns the current set of metrics for a container
	Metrics() (Metrics, error)

	// Properties returns the current set of properties
	Properties() (Properties, error)

	// Property returns the value of the property with the specified name.
	//
	// Errors:
	// * When the property does not exist on the container.
	Property(name string) (string, error)

	// Set a named property on a container to a specified value.
	//
	// Errors:
	// * None.
	SetProperty(name string, value string) error

	// Remove a property with the specified name from a container.
	//
	// Errors:
	// * None.
	RemoveProperty(name string) error
}

// ProcessSpec contains parameters for running a script inside a container.
type ProcessSpec struct {
	// Path to command to execute.
	Path string `json:"path,omitempty"`

	// Arguments to pass to command.
	Args []string `json:"args,omitempty"`

	// Environment variables.
	Env []string `json:"env,omitempty"`

	// Working directory (default: home directory).
	Dir string `json:"dir,omitempty"`

	// The name of a user in the container to run the process as.
	User string `json:"user,omitempty"`

	// Resource limits
	Limits ResourceLimits `json:"rlimits,omitempty"`

	// Execute with a TTY for stdio.
	TTY *TTYSpec `json:"tty,omitempty"`
}

type TTYSpec struct {
	WindowSize *WindowSize `json:"window_size,omitempty"`
}

type WindowSize struct {
	Columns int `json:"columns,omitempty"`
	Rows    int `json:"rows,omitempty"`
}

type ProcessIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

//go:generate counterfeiter . Process

type Process interface {
	ID() uint32
	Wait() (int, error)
	SetTTY(TTYSpec) error
	Signal(Signal) error
}

type Signal int

const (
	SignalTerminate Signal = iota
	SignalKill
)

type PortMapping struct {
	HostPort      uint32
	ContainerPort uint32
}

type StreamInSpec struct {
	Path      string
	User      string
	TarStream io.Reader
}

type StreamOutSpec struct {
	Path string
	User string
}

// ContainerInfo holds information about a container.
type ContainerInfo struct {
	State         string        // Either "active" or "stopped".
	Events        []string      // List of events that occurred for the container. It currently includes only "oom" (Out Of Memory) event if it occurred.
	HostIP        string        // The IP address of the gateway which controls the host side of the container's virtual ethernet pair.
	ContainerIP   string        // The IP address of the container side of the container's virtual ethernet pair.
	ExternalIP    string        //
	ContainerPath string        // The path to the directory holding the container's files (both its control scripts and filesystem).
	ProcessIDs    []uint32      // List of running processes.
	Properties    Properties    // List of properties defined for the container.
	MappedPorts   []PortMapping //
}

func NewError(msg string) *Error {
	return &Error{msg}
}

type Error struct {
	ErrorMsg string `json:"error_msg"`
}

func (e *Error) Error() string {
	return e.ErrorMsg
}

type ContainerInfoEntry struct {
	Info ContainerInfo
	Err  *Error
}

type Metrics struct {
	MemoryStat  ContainerMemoryStat
	CPUStat     ContainerCPUStat
	DiskStat    ContainerDiskStat
	NetworkStat ContainerNetworkStat
}

type ContainerMetricsEntry struct {
	Metrics Metrics
	Err     *Error
}

type ContainerMemoryStat struct {
	Cache                   uint64
	Rss                     uint64
	MappedFile              uint64
	Pgpgin                  uint64
	Pgpgout                 uint64
	Swap                    uint64
	Pgfault                 uint64
	Pgmajfault              uint64
	InactiveAnon            uint64
	ActiveAnon              uint64
	InactiveFile            uint64
	ActiveFile              uint64
	Unevictable             uint64
	HierarchicalMemoryLimit uint64
	HierarchicalMemswLimit  uint64
	TotalCache              uint64
	TotalRss                uint64
	TotalMappedFile         uint64
	TotalPgpgin             uint64
	TotalPgpgout            uint64
	TotalSwap               uint64
	TotalPgfault            uint64
	TotalPgmajfault         uint64
	TotalInactiveAnon       uint64
	TotalActiveAnon         uint64
	TotalInactiveFile       uint64
	TotalActiveFile         uint64
	TotalUnevictable        uint64
	// A memory usage total which reports memory usage in the same way that limits are enforced.
	// This value includes memory consumed by nested containers.
	TotalUsageTowardLimit uint64
}

type ContainerCPUStat struct {
	Usage  uint64
	User   uint64
	System uint64
}

type ContainerDiskStat struct {
	TotalBytesUsed      uint64
	TotalInodesUsed     uint64
	ExclusiveBytesUsed  uint64
	ExclusiveInodesUsed uint64
}

type ContainerBandwidthStat struct {
	InRate   uint64
	InBurst  uint64
	OutRate  uint64
	OutBurst uint64
}

type ContainerNetworkStat struct {
	RxBytes uint64
	TxBytes uint64
}

type BandwidthLimits struct {
	RateInBytesPerSecond      uint64 `json:"rate,omitempty"`
	BurstRateInBytesPerSecond uint64 `json:"burst,omitempty"`
}

type DiskLimits struct {
	InodeSoft uint64 `json:"inode_soft,omitempty"`
	InodeHard uint64 `json:"inode_hard,omitempty"`

	ByteSoft uint64 `json:"byte_soft,omitempty"`
	ByteHard uint64 `json:"byte_hard,omitempty"`

	Scope DiskLimitScope `json:"scope,omitempty"`
}

type MemoryLimits struct {
	//	Memory usage limit in bytes.
	LimitInBytes uint64 `json:"limit_in_bytes,omitempty"`
}

type CPULimits struct {
	LimitInShares uint64 `json:"limit_in_shares,omitempty"`
}

// Resource limits.
//
// Please refer to the manual page of getrlimit for a description of the individual fields:
// http://www.kernel.org/doc/man-pages/online/pages/man2/getrlimit.2.html
type ResourceLimits struct {
	As         *uint64 `json:"as,omitempty"`
	Core       *uint64 `json:"core,omitempty"`
	Cpu        *uint64 `json:"cpu,omitempty"`
	Data       *uint64 `json:"data,omitempty"`
	Fsize      *uint64 `json:"fsize,omitempty"`
	Locks      *uint64 `json:"locks,omitempty"`
	Memlock    *uint64 `json:"memlock,omitempty"`
	Msgqueue   *uint64 `json:"msgqueue,omitempty"`
	Nice       *uint64 `json:"nice,omitempty"`
	Nofile     *uint64 `json:"nofile,omitempty"`
	Nproc      *uint64 `json:"nproc,omitempty"`
	Rss        *uint64 `json:"rss,omitempty"`
	Rtprio     *uint64 `json:"rtprio,omitempty"`
	Sigpending *uint64 `json:"sigpending,omitempty"`
	Stack      *uint64 `json:"stack,omitempty"`
}

type DiskLimitScope uint8

const DiskLimitScopeTotal DiskLimitScope = 0
const DiskLimitScopeExclusive DiskLimitScope = 1
