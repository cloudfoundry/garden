package linux_backend

import (
	"github.com/vito/garden/backend"
)

type ContainerSnapshot struct {
	ID     string
	Handle string

	State  string
	Events []string

	Limits LimitsSnapshot

	Resources ResourcesSnapshot

	Jobs []JobSnapshot

	NetIns  []NetInSpec
	NetOuts []NetOutSpec
}

type LimitsSnapshot struct {
	Memory    *backend.MemoryLimits
	Disk      *backend.DiskLimits
	Bandwidth *backend.BandwidthLimits
	CPU       *backend.CPULimits
}

type ResourcesSnapshot struct {
	UID     uint32
	Network string
	Ports   []uint32
}

type JobSnapshot struct {
	ID            uint32
	DiscardOutput bool
}
