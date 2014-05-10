package fake_backend

import (
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"
)

const defaultBytes = 1024 * 1024 * 1024

type FakeBackend struct {
	Started    bool
	StartError error

	Stopped bool

	CreateResult    *FakeContainer
	CreateError     error
	DestroyError    error
	ContainersError error

	CreatedContainers   map[string]*FakeContainer
	DestroyedContainers []string

	ContainersFilters []warden.Properties

	CapacityError  error
	CapacityResult warden.Capacity

	sync.RWMutex
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
		CapacityResult: warden.Capacity{
			MemoryInBytes: defaultBytes,
			DiskInBytes:   defaultBytes,
			MaxContainers: 1024,
		},
	}
}

func (b *FakeBackend) Start() error {
	if b.StartError != nil {
		return b.StartError
	}

	b.Started = true

	return nil
}

func (b *FakeBackend) Stop() {
	b.Stopped = true
}

func (b *FakeBackend) Capacity() (warden.Capacity, error) {
	if b.CapacityError != nil {
		return warden.Capacity{}, b.CapacityError
	}

	return b.CapacityResult, nil
}

func (b *FakeBackend) Create(spec warden.ContainerSpec) (warden.Container, error) {
	if b.CreateError != nil {
		return nil, b.CreateError
	}

	var container *FakeContainer

	if b.CreateResult != nil {
		container = b.CreateResult
	} else {
		container = NewFakeContainer(spec)
	}

	b.Lock()
	defer b.Unlock()

	b.CreatedContainers[container.Handle()] = container

	return container, nil
}

func (b *FakeBackend) Destroy(handle string) error {
	if b.DestroyError != nil {
		return b.DestroyError
	}

	b.Lock()
	defer b.Unlock()

	delete(b.CreatedContainers, handle)

	b.DestroyedContainers = append(b.DestroyedContainers, handle)

	return nil
}

func (b *FakeBackend) Containers(properties warden.Properties) ([]warden.Container, error) {
	if b.ContainersError != nil {
		return nil, b.ContainersError
	}

	b.Lock()
	defer b.Unlock()

	b.ContainersFilters = append(b.ContainersFilters, properties)

	containers := []warden.Container{}
	for _, c := range b.CreatedContainers {
		if containerHasProperties(c, properties) {
			containers = append(containers, c)
		}
	}

	return containers, nil
}

func (b *FakeBackend) Lookup(handle string) (warden.Container, error) {
	b.RLock()
	defer b.RUnlock()

	container, found := b.CreatedContainers[handle]
	if !found {
		return nil, UnknownHandleError{handle}
	}

	return container, nil
}

func (b *FakeBackend) GraceTime(container warden.Container) time.Duration {
	return container.(*FakeContainer).GraceTime()
}

func containerHasProperties(container *FakeContainer, properties warden.Properties) bool {
	containerProps := container.Properties()

	for key, val := range properties {
		cval, ok := containerProps[key]
		if !ok {
			return false
		}

		if cval != val {
			return false
		}
	}

	return true
}
