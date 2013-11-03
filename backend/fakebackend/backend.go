package fakebackend

import (
	"github.com/vito/garden/backend"
)

type FakeBackend struct {
	ContainerCreationError error
}

func New() *FakeBackend {
	return &FakeBackend{}
}

func (b *FakeBackend) Create(spec backend.ContainerSpec) (backend.Container, error) {
	return &FakeContainer{handle: spec.Handle}, b.ContainerCreationError
}

func (b *FakeBackend) Containers() ([]backend.Container, error) {
	return []backend.Container{}, nil
}
