package fakebackend

import (
	"github.com/vito/garden/backend"
)

type FakeBackend struct{}

func New() *FakeBackend {
	return &FakeBackend{}
}

func (b *FakeBackend) Create(backend.ContainerSpec) (backend.Container, error) {
	return &FakeContainer{}, nil
}

func (b *FakeBackend) Containers() ([]backend.Container, error) {
	return []backend.Container{}, nil
}
