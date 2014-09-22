package fake_api_client

import (
	"github.com/cloudfoundry-incubator/garden/api"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fakes"
)

type FakeClient struct {
	api.Client

	Connection *fakes.FakeConnection
}

func New() *FakeClient {
	connection := new(fakes.FakeConnection)

	return &FakeClient{
		Connection: connection,

		Client: client.New(connection),
	}
}
