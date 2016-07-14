package client

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client/connection"
)

type Client interface {
	garden.Client
}

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

func (client *client) Capacity() (garden.Capacity, error) {
	return client.connection.Capacity()
}

func (client *client) Create(spec garden.ContainerSpec) (garden.Container, error) {
	handle, err := client.connection.Create(spec)
	if err != nil {
		return nil, err
	}

	return newContainer(handle, client.connection), nil
}

func (client *client) Containers(properties garden.Properties) ([]garden.Container, error) {
	handles, err := client.connection.List(properties)
	if err != nil {
		return nil, err
	}

	containers := []garden.Container{}
	for _, handle := range handles {
		containers = append(containers, newContainer(handle, client.connection))
	}

	return containers, nil
}

func (client *client) Destroy(handle string) error {
	err := client.connection.Destroy(handle)

	return err
}

func (client *client) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return client.connection.BulkInfo(handles)
}

func (client *client) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return client.connection.BulkMetrics(handles)
}

func (client *client) Lookup(handle string) (garden.Container, error) {
	handles, err := client.connection.List(nil)
	if err != nil {
		return nil, err
	}

	for _, h := range handles {
		if h == handle {
			return newContainer(handle, client.connection), nil
		}
	}

	return nil, garden.ContainerNotFoundError{Handle: handle}
}
