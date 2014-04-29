package warden

import (
	"time"
)

type Client interface {
	Create(ContainerSpec) (Container, error)
	Destroy(handle string) error
	Containers(Properties) ([]Container, error)
	Lookup(handle string) (Container, error)
}

type ContainerSpec struct {
	Handle     string
	GraceTime  time.Duration
	RootFSPath string
	BindMounts []BindMount
	Network    string
	Properties Properties
}

type BindMount struct {
	SrcPath string
	DstPath string
	Mode    BindMountMode
	Origin  BindMountOrigin
}

type Properties map[string]string

type BindMountMode uint8

const BindMountModeRO BindMountMode = 0
const BindMountModeRW BindMountMode = 1

type BindMountOrigin uint8

const BindMountOriginHost BindMountOrigin = 0
const BindMountOriginContainer BindMountOrigin = 1
