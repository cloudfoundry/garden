package client_test

import (
	"errors"
	"sync"

	"github.com/cloudfoundry-incubator/garden/client/connection"
)

type FakeConnectionProvider struct {
	Connection connection.Connection
}

func (provider *FakeConnectionProvider) ProvideConnection() (connection.Connection, error) {
	return provider.Connection, nil
}

type ManyConnectionProvider struct {
	Connections []connection.Connection

	index int

	sync.Mutex
}

func (provider *ManyConnectionProvider) ProvideConnection() (connection.Connection, error) {
	provider.Lock()
	defer provider.Unlock()

	if provider.index == len(provider.Connections) {
		return nil, errors.New("exhausted fake connections")
	}

	connection := provider.Connections[provider.index]

	provider.index++

	return connection, nil
}
