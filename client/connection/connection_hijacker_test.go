package connection_test

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/garden/routes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"
)

var _ = Describe("ConnectionHijacker", func() {
	Describe("constructing hijacker with a dialer", func() {
		var hijackStreamer connection.HijackStreamer

		BeforeEach(func() {
			dialer := func(string, string) (net.Conn, error) {
				return nil, errors.New("oh no I am hijacked")
			}
			hijackStreamer = connection.NewHijackStreamerWithDialer(dialer)
		})

		Context("when Hijack is called", func() {
			It("should use the dialer", func() {
				_, _, err := hijackStreamer.Hijack(
					routes.Run,
					new(bytes.Buffer),
					rata.Params{
						"handle": "some-test-handle",
					},
					nil,
					"application/json",
				)
				Expect(err).To(MatchError("oh no I am hijacked"))
			})
		})

		Context("when Stream is called", func() {
			It("should use the dialer", func() {
				_, err := hijackStreamer.Stream(
					routes.Run,
					new(bytes.Buffer),
					rata.Params{
						"handle": "some-test-handle",
					},
					nil,
					"application/json",
				)

				pathError, ok := err.(*url.Error)
				Expect(ok).To(BeTrue())
				Expect(pathError.Err).To(MatchError("oh no I am hijacked"))
			})
		})
	})

	Describe("constructing hijacker with a headers", func() {
		var (
			hijackStreamer connection.HijackStreamer
			fakeServer     *ghttp.Server
		)

		BeforeEach(func() {
			fakeServer = ghttp.NewServer()
			fakeServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					ghttp.RespondWith(200, "{}"),
				),
			)
			hijackStreamer = connection.NewHijackStreamerWithHeaders("tcp", fakeServer.HTTPTestServer.Listener.Addr().String(), http.Header{"some-header": []string{"some-value"}})
		})

		AfterEach(func() {
			fakeServer.Close()
		})

		Context("when Hijack is called", func() {
			It("should use the dialer", func() {
				_, _, err := hijackStreamer.Hijack(
					routes.Ping,
					new(bytes.Buffer),
					nil,
					nil,
					"application/json",
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
				req := fakeServer.ReceivedRequests()[0]
				Expect(req.Header.Values("some-header")).To(Equal([]string{"some-value"}))
			})
		})

		Context("when Stream is called", func() {
			It("should use the dialer", func() {
				_, err := hijackStreamer.Stream(
					routes.Ping,
					new(bytes.Buffer),
					nil,
					nil,
					"application/json",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeServer.ReceivedRequests()).To(HaveLen(1))
				req := fakeServer.ReceivedRequests()[0]
				Expect(req.Header.Values("some-header")).To(Equal([]string{"some-value"}))
			})
		})
	})

})
