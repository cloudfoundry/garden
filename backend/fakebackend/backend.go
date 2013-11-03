package fakebackend

import (
	"github.com/vito/garden/backend"
)

type FakeBackend struct {
	ContainerCreationError error

	CreatedContainers map[string]*FakeContainer
}

func New() *FakeBackend {
	return &FakeBackend{
		CreatedContainers: make(map[string]*FakeContainer),
	}
}

func (b *FakeBackend) Create(spec backend.ContainerSpec) (backend.Container, error) {
	if b.ContainerCreationError != nil {
		return nil, b.ContainerCreationError
	}

	container := &FakeContainer{Spec: spec}

	b.CreatedContainers[container.Handle()] = container

	return container, nil
}

func (b *FakeBackend) Containers() (containers []backend.Container, err error) {
	for _, c := range b.CreatedContainers {
		containers = append(containers, c)
	}

	return
}
