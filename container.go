package garden

import (
	"io"
	"time"
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

	// Returns the current bandwidth limits set for the container.
	CurrentBandwidthLimits() (BandwidthLimits, error)

	// Returns the current CPU limts set for the container.
	CurrentCPULimits() (CPULimits, error)

	// Returns the current disk limts set for the container.
	CurrentDiskLimits() (DiskLimits, error)

	// Returns the current memory limts set for the container.
	CurrentMemoryLimits() (MemoryLimits, error)

	// Map a port on the host to a port in the container so that traffic to the
	// host port is forwarded to the container port. This is deprecated in
	// favour of passing NetIn configuration in the ContainerSpec at creation
	// time.
	//
	// If a host port is not given, a port will be acquired from the server's port
	// pool.
	//
	// If a container port is not given, the port will be the same as the
	// host port.
	//
	// The resulting host and container ports are returned in that order.
	//
	// Errors:
	// * When no port can be acquired from the server's port pool.
	NetIn(hostPort, containerPort uint32) (uint32, uint32, error)

	// Whitelist outbound network traffic. This is deprecated in favour of passing
	// NetOut configuration in the ContainerSpec at creation time.
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

	// A Bulk call for NetOut. This is deprecated in favour of passing
	// NetOut configuration in the ContainerSpec at creation time.
	//
	// Errors:
	// * An error is returned if any of the NetOut calls fail.
	BulkNetOut(netOutRules []NetOutRule) error

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
	Attach(processID string, io ProcessIO) (Process, error)

	// Metrics returns the current set of metrics for a container
	Metrics() (Metrics, error)

	// Sets the grace time.
	SetGraceTime(graceTime time.Duration) error

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
	// ID for the process. If empty, an ID will be generated.
	ID string `json:"id,omitempty"`

	// Path to command to execute.
	Path string `json:"path,omitempty"`

	// Arguments to pass to command.
	Args []string `json:"args,omitempty"`

	// Environment variables.
	Env []string `json:"env,omitempty"`

	// Working directory (default: home directory).
	Dir string `json:"dir,omitempty"`

	// The name of a user in the container to run the process as.
	// This must either be a username, or uid:gid.
	User string `json:"user,omitempty"`

	// Resource limits
	Limits ResourceLimits `json:"rlimits,omitempty"`

	// Limits to be applied to the newly created process
	OverrideContainerLimits *ProcessLimits `json:"limits,omitempty"`

	// Execute with a TTY for stdio.
	TTY *TTYSpec `json:"tty,omitempty"`

	// Execute process in own root filesystem, different from the other processes
	// in the container.
	Image ImageRef `json:"image,omitempty"`

	// Bind mounts to be applied to the process's filesystem
	// An error is returned if ProcessSpec.Image is not also set.
	BindMounts []BindMount `json:"bind_mounts,omitempty"`
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
	ID() string
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
	ProcessIDs    []string      // List of running processes.
	Properties    Properties    // List of properties defined for the container.
	MappedPorts   []PortMapping //
}

type ContainerInfoEntry struct {
	Info ContainerInfo
	Err  *Error
}

type Metrics struct {
	MemoryStat     ContainerMemoryStat
	CPUStat        ContainerCPUStat
	DiskStat       ContainerDiskStat
	NetworkStat    ContainerNetworkStat
	PidStat        ContainerPidStat
	Age            time.Duration
	CPUEntitlement uint64
}

type ContainerMetricsEntry struct {
	Metrics Metrics
	Err     *Error
}

type ContainerMemoryStat struct {
	ActiveAnon              uint64 `json:"active_anon"`
	ActiveFile              uint64 `json:"active_file"`
	Cache                   uint64 `json:"cache"`
	HierarchicalMemoryLimit uint64 `json:"hierarchical_memory_limit"`
	InactiveAnon            uint64 `json:"inactive_anon"`
	InactiveFile            uint64 `json:"inactive_file"`
	MappedFile              uint64 `json:"mapped_file"`
	Pgfault                 uint64 `json:"pgfault"`
	Pgmajfault              uint64 `json:"pgmajfault"`
	Pgpgin                  uint64 `json:"pgpgin"`
	Pgpgout                 uint64 `json:"pgpgout"`
	Rss                     uint64 `json:"rss"`
	TotalActiveAnon         uint64 `json:"total_active_anon"`
	TotalActiveFile         uint64 `json:"total_active_file"`
	TotalCache              uint64 `json:"total_cache"`
	TotalInactiveAnon       uint64 `json:"total_inactive_anon"`
	TotalInactiveFile       uint64 `json:"total_inactive_file"`
	TotalMappedFile         uint64 `json:"total_mapped_file"`
	TotalPgfault            uint64 `json:"total_pgfault"`
	TotalPgmajfault         uint64 `json:"total_pgmajfault"`
	TotalPgpgin             uint64 `json:"total_pgpgin"`
	TotalPgpgout            uint64 `json:"total_pgpgout"`
	TotalRss                uint64 `json:"total_rss"`
	TotalUnevictable        uint64 `json:"total_unevictable"`
	Unevictable             uint64 `json:"unevictable"`
	Swap                    uint64 `json:"swap"`
	HierarchicalMemswLimit  uint64 `json:"hierarchical_memsw_limit"`
	TotalSwap               uint64 `json:"total_swap"`
	// A memory usage total which reports memory usage in the same way that limits are enforced.
	// This value includes memory consumed by nested containers.
	TotalUsageTowardLimit uint64
}

type ContainerCPUStat struct {
	Usage  uint64
	User   uint64
	System uint64
}

type ContainerPidStat struct {
	Current uint64
	Max     uint64
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

type ProcessLimits struct {
	CPU    CPULimits    `json:"cpu_limits,omitempty"`
	Memory MemoryLimits `json:"memory_limits,omitempty"`
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
	Weight uint64 `json:"weight,omitempty"`
	// Deprecated: Use Weight instead.
	LimitInShares uint64 `json:"limit_in_shares,omitempty"`
}

type PidLimits struct {
	// Limits the number of pids a container may create before new forks or clones are disallowed to processes in the container.
	// Note: this may only be enforced when a process attempts to fork, so it does not guarantee that a new container.Run(ProcessSpec)
	// will not succeed even if the limit has been exceeded, but the process will not be able to spawn further processes or threads.
	Max uint64 `json:"max,omitempty"`
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
