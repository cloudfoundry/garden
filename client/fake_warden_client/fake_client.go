package fake_warden_client

import (
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fakes"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type FakeClient struct {
	warden.Client

	Connection *fakes.FakeConnection
}

func New() *FakeClient {
	connection := new(fakes.FakeConnection)

	return &FakeClient{
		Connection: connection,

		Client: client.New(connection),
	}
}
