package backend

import (
	"time"
)

type Container interface {
	Destroy() error
	Stop(background bool, kill bool) error

	Info() (ContainerInfo, error)

	CopyIn(srcPath, dstPath string) error
	CopyOut(srcPath, dstPath, owner string) error

	LimitBandwidth(limits BandwidthLimits) (BandwidthLimits, error)
	LimitDisk(limits DiskLimits) (DiskLimits, error)
	LimitMemory(limits MemoryLimits) (MemoryLimits, error)

	Spawn(JobSpec) (uint32, error)
	Stream(jobID uint32) (<-chan JobStream, error)
	Link(jobID uint32) (JobResult, error)
	Run(JobSpec) (JobResult, error)

	NetIn(hostPort, containerPort uint32) (uint32, uint32, error)
	NetOut(network string, port uint32) error
}

type ContainerSpec struct {
	Handle     string
	GraceTime  time.Duration
	RootFSPath string
	BindMounts []BindMount
	Network    string
}

type JobSpec struct {
	Script        string
	Priveleged    bool
	Limits        ResourceLimits
	DiscardOutput bool
}

type JobResult struct {
	ExitStatus uint32
	Stdout     string
	Stderr     string
	Info       ContainerInfo
}

type JobStream struct {
	Name       string
	Data       []byte
	ExitStatus uint32
	Info       ContainerInfo
}

type BindMount struct {
	SrcPath string
	DstPath string
	Mode    BindMountMode
}

type BindMountMode uint8

const BindMountModeRO BindMountMode = 0
const BindMountModeRW BindMountMode = 1

type ContainerInfo struct{}

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

type ResourceLimits struct {
	As         uint64
	Core       uint64
	Cpu        uint64
	Data       uint64
	Fsize      uint64
	Locks      uint64
	Memlock    uint64
	Msgqueue   uint64
	Nice       uint64
	Nofile     uint64
	Nproc      uint64
	Rss        uint64
	Rtprio     uint64
	Sigpending uint64
	Stack      uint64
}
