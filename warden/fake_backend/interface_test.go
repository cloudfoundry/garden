package fake_backend

import (
	"testing"

	"github.com/cloudfoundry-incubator/garden/warden"
)

func FunctionTakingBackend(warden.Backend) {

}

func FunctionTakingContainer(warden.Container) {

}

func TestFakeBackendConformsToInterface(*testing.T) {
	FunctionTakingBackend(&FakeBackend{})
}

func TestFakeContainerConformsToInterface(*testing.T) {
	FunctionTakingContainer(&FakeContainer{})
}
