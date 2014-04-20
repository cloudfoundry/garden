package fake_backend

import (
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"
)

type SlowFakeBackend struct {
	FakeBackend

	delay time.Duration
}

func NewSlow(delay time.Duration) *SlowFakeBackend {
	return &SlowFakeBackend{
		FakeBackend: *New(),

		delay: delay,
	}
}

func (b *SlowFakeBackend) Create(spec warden.ContainerSpec) (warden.Container, error) {
	time.Sleep(b.delay)

	return b.FakeBackend.Create(spec)
}

func (b *SlowFakeBackend) Lookup(handle string) (warden.Container, error) {
	time.Sleep(b.delay)

	return b.FakeBackend.Lookup(handle)
}
