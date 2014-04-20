package warden

type Container interface {
	Handle() string

	Stop(kill bool) error

	Info() (ContainerInfo, error)

	CopyIn(srcPath, dstPath string) error
	CopyOut(srcPath, dstPath, owner string) error

	LimitBandwidth(limits BandwidthLimits) error
	CurrentBandwidthLimits() (BandwidthLimits, error)

	LimitCPU(limits CPULimits) error
	CurrentCPULimits() (CPULimits, error)

	LimitDisk(limits DiskLimits) error
	CurrentDiskLimits() (DiskLimits, error)

	LimitMemory(limits MemoryLimits) error
	CurrentMemoryLimits() (MemoryLimits, error)

	Run(ProcessSpec) (uint32, <-chan ProcessStream, error)
	Attach(processID uint32) (<-chan ProcessStream, error)

	NetIn(hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(network string, port uint32) error
}

type ProcessSpec struct {
	Script               string
	Privileged           bool
	Limits               ResourceLimits
	EnvironmentVariables []EnvironmentVariable
}

type ProcessStream struct {
	Source     ProcessStreamSource
	Data       []byte
	ExitStatus *uint32
}

type ProcessStreamSource int32

const (
	ProcessStreamSourceInvalid ProcessStreamSource = iota
	ProcessStreamSourceStdin
	ProcessStreamSourceStdout
	ProcessStreamSourceStderr
)

type ContainerInfo struct {
	State         string
	Events        []string
	HostIP        string
	ContainerIP   string
	ContainerPath string
	ProcessIDs    []uint32
	MemoryStat    ContainerMemoryStat
	CPUStat       ContainerCPUStat
	DiskStat      ContainerDiskStat
	BandwidthStat ContainerBandwidthStat
	Properties    Properties
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
}

type ContainerCPUStat struct {
	Usage  uint64
	User   uint64
	System uint64
}

type ContainerDiskStat struct {
	BytesUsed  uint64
	InodesUsed uint64
}

type ContainerBandwidthStat struct {
	InRate   uint64
	InBurst  uint64
	OutRate  uint64
	OutBurst uint64
}

type BandwidthLimits struct {
	RateInBytesPerSecond      uint64
	BurstRateInBytesPerSecond uint64
}

type DiskLimits struct {
	BlockLimit uint64
	Block      uint64
	BlockSoft  uint64
	BlockHard  uint64

	InodeLimit uint64
	Inode      uint64
	InodeSoft  uint64
	InodeHard  uint64

	ByteLimit uint64
	Byte      uint64
	ByteSoft  uint64
	ByteHard  uint64
}

type MemoryLimits struct {
	LimitInBytes uint64
}

type CPULimits struct {
	LimitInShares uint64
}

type EnvironmentVariable struct {
	Key   string
	Value string
}

type ResourceLimits struct {
	As         *uint64
	Core       *uint64
	Cpu        *uint64
	Data       *uint64
	Fsize      *uint64
	Locks      *uint64
	Memlock    *uint64
	Msgqueue   *uint64
	Nice       *uint64
	Nofile     *uint64
	Nproc      *uint64
	Rss        *uint64
	Rtprio     *uint64
	Sigpending *uint64
	Stack      *uint64
}
