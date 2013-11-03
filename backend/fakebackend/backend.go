package fakebackend

import (
	"errors"

	"github.com/vito/garden/backend"
)

type FakeBackend struct {
	CreateError     error
	DestroyError    error
	ContainersError error

	CreatedContainers map[string]*FakeContainer
}

func New() *FakeBackend {
	return &FakeBackend{
		CreatedContainers: make(map[string]*FakeContainer),
	}
}

func (b *FakeBackend) Create(spec backend.ContainerSpec) (backend.Container, error) {
	if b.CreateError != nil {
		return nil, b.CreateError
	}

	container := &FakeContainer{Spec: spec}

	b.CreatedContainers[container.Handle()] = container

	return container, nil
}

func (b *FakeBackend) Destroy(handle string) error {
	if b.DestroyError != nil {
		return b.DestroyError
	}

	container, found := b.CreatedContainers[handle]
	if !found {
		return errors.New("unknown handle: " + handle)
	}

	err := container.Destroy()
	if err != nil {
		return err
	}

	delete(b.CreatedContainers, handle)

	return nil
}

func (b *FakeBackend) Containers() (containers []backend.Container, err error) {
	if b.ContainersError != nil {
		err = b.ContainersError
		return
	}

	for _, c := range b.CreatedContainers {
		containers = append(containers, c)
	}

	return
}
