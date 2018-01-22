package server_test

import (
	"testing"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/garden/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

func listenAndServe(apiServer *server.GardenServer, network, addr string) {
	ExpectWithOffset(1, apiServer.SetupBomberman()).To(Succeed())
	go func() {
		defer GinkgoRecover()
		Expect(apiServer.ListenAndServe()).To(Succeed())
	}()

	apiClient := client.New(connection.New(network, addr))
	EventuallyWithOffset(1, apiClient.Ping, "2s").Should(Succeed())
}
