package client

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type Client interface {
	warden.Client
}

var ErrContainerNotFound = errors.New("container not found")

type client struct {
	connection connection.Connection
}

func New(connection connection.Connection) Client {
	return &client{
		connection: connection,
	}
}

func (client *client) Ping() error {
	return client.connection.Ping()
}

func (client *client) Capacity() (warden.Capacity, error) {
	return client.connection.Capacity()
}

func (client *client) Create(spec warden.ContainerSpec) (warden.Container, error) {
	handle, err := client.connection.Create(spec)
	if err != nil {
		return nil, err
	}

	return newContainer(handle, client.connection), nil
}

func (client *client) Containers(properties warden.Properties) ([]warden.Container, error) {
	handles, err := client.connection.List(properties)
	if err != nil {
		return nil, err
	}

	containers := []warden.Container{}
	for _, handle := range handles {
		containers = append(containers, newContainer(handle, client.connection))
	}

	return containers, nil
}

func (client *client) Destroy(handle string) error {
	return client.connection.Destroy(handle)
}

func (client *client) Lookup(handle string) (warden.Container, error) {
	handles, err := client.connection.List(nil)
	if err != nil {
		return nil, err
	}

	for _, h := range handles {
		if h == handle {
			return newContainer(handle, client.connection), nil
		}
	}

	return nil, fmt.Errorf("container not found: %s", handle)
}
