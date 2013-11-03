package backend

import (
	"time"
)

type Backend interface {
	Create(ContainerSpec) (Container, error)
	Containers() ([]Container, error)
}

type ContainerSpec struct {
	Handle     string
	GraceTime  time.Duration
	RootFSPath string
	BindMounts []BindMount
	Network    string
}
