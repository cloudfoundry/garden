package fake_backend

import (
	"io"

	"github.com/vito/garden/backend"
)

type FakeBackend struct {
	CreateResult    *FakeContainer
	CreateError     error
	RestoreError    error
	DestroyError    error
	ContainersError error

	CreatedContainers  map[string]*FakeContainer
	RestoredContainers []io.Reader
}

type UnknownHandleError struct {
	Handle string
}

func (e UnknownHandleError) Error() string {
	return "unknown handle: " + e.Handle
}

func New() *FakeBackend {
	return &FakeBackend{
		CreatedContainers: make(map[string]*FakeContainer),
	}
}

func (b *FakeBackend) Setup() error {
	return nil
}

func (b *FakeBackend) Create(spec backend.ContainerSpec) (backend.Container, error) {
	if b.CreateError != nil {
		return nil, b.CreateError
	}

	var container *FakeContainer

	if b.CreateResult != nil {
		container = b.CreateResult
	} else {
		container = NewFakeContainer(spec)
	}

	b.CreatedContainers[container.Handle()] = container

	return container, nil
}

func (b *FakeBackend) Restore(snapshot io.Reader) error {
	if b.RestoreError != nil {
		return b.RestoreError
	}

	b.RestoredContainers = append(b.RestoredContainers, snapshot)

	return nil
}

func (b *FakeBackend) Destroy(handle string) error {
	if b.DestroyError != nil {
		return b.DestroyError
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

func (b *FakeBackend) Lookup(handle string) (backend.Container, error) {
	container, found := b.CreatedContainers[handle]
	if !found {
		return nil, UnknownHandleError{handle}
	}

	return container, nil
}
