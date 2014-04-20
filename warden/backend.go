package warden

import "time"

type Backend interface {
	Start() error
	Stop()

	Create(ContainerSpec) (BackendContainer, error)
	Destroy(handle string) error
	Containers() ([]BackendContainer, error)
	Lookup(handle string) (BackendContainer, error)
}

type BackendContainer interface {
	Container

	GraceTime() time.Duration
	Properties() Properties
}
