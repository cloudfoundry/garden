package transport_test

import (
	"bufio"
	"bytes"

	"code.google.com/p/gogoprotobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/transport"
)

var _ = Describe("Reading response messages over the wire", func() {
	Context("when a message of the expected type is received", func() {
		It("populates the response object and returns no error", func() {
			var pingResponse protocol.PingResponse

			err := transport.ReadMessage(
				bufio.NewReader(protocol.Messages(&protocol.PingRequest{})),
				&pingResponse,
			)

			Ω(err).ShouldNot(HaveOccurred())

			Ω(pingResponse).Should(Equal(protocol.PingResponse{}))
		})
	})

	Context("when the connection is broken", func() {
		It("returns an error", func() {
			var dummyResponse protocol.PingResponse

			payload := protocol.Messages(&protocol.PingRequest{})

			bogusPayload := bufio.NewReader(
				bytes.NewBuffer(payload.Bytes()[0 : payload.Len()-1]),
			)

			err := transport.ReadMessage(bogusPayload, &dummyResponse)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when an error is received", func() {
		It("returns a WardenError", func() {
			var dummyResponse protocol.PingResponse

			err := transport.ReadMessage(
				bufio.NewReader(protocol.Messages(&protocol.ErrorResponse{
					Message: proto.String("some message"),
					Data:    proto.String("some data"),
					Backtrace: []string{
						"backtrace line 1",
						"backtrace line 2",
					},
				})),
				&dummyResponse,
			)

			Ω(err).Should(Equal(
				&transport.WardenError{
					Message: "some message",
					Data:    "some data",
					Backtrace: []string{
						"backtrace line 1",
						"backtrace line 2",
					},
				},
			))

		})
	})

	Context("when a message of the wrong type is received", func() {
		It("returns a TypeMismatchError", func() {
			var dummyResponse protocol.PingResponse

			actualResponse := &protocol.StopResponse{}

			err := transport.ReadMessage(
				bufio.NewReader(protocol.Messages(actualResponse)),
				&dummyResponse,
			)

			Ω(err).Should(Equal(
				&transport.TypeMismatchError{
					Expected: protocol.TypeForMessage(&dummyResponse),
					Received: protocol.TypeForMessage(actualResponse),
				},
			))

		})
	})
})
