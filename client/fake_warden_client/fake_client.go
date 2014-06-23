package fake_warden_client

import (
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fake_connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type FakeClient struct {
	warden.Client

	Connection *fake_connection.FakeConnection
}

func New() *FakeClient {
	connection := fake_connection.New()

	return &FakeClient{
		Connection: connection,

		Client: client.New(connection),
	}
}
