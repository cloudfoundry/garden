package server_test

import (
	"io/ioutil"
	"net"
	"os"
	"path"

	"code.google.com/p/gogoprotobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend/fakebackend"
	"github.com/vito/garden/messagereader"
	protocol "github.com/vito/garden/protocol"
	"github.com/vito/garden/server"
)

var _ = Describe("The Warden server", func() {
	It("listens on the given socket path", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccured())

		socketPath := path.Join(tmpdir, "warden.sock")

		wardenServer := server.New(socketPath, fakebackend.New())

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccured())

		Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccured())
	})

	Context("when starting fails", func() {
		It("returns the error", func() {
			tmpfile, err := ioutil.TempFile(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccured())

			wardenServer := server.New(
				// weird scenario: /foo/X/warden.sock with X being a file
				path.Join(tmpfile.Name(), "warden.sock"),
				fakebackend.New(),
			)

			err = wardenServer.Start()
			Expect(err).To(HaveOccured())
		})
	})

	Context("when a client connects", func() {
		var socketPath string

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccured())

			socketPath = path.Join(tmpdir, "warden.sock")

			wardenServer := server.New(socketPath, fakebackend.New())

			err = wardenServer.Start()
			Expect(err).ToNot(HaveOccured())

			Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccured())
		})

		Context("and the client sends a PingRequest", func() {
			It("sends a PongResponse", func(done Done) {
				conn, err := net.Dial("unix", socketPath)
				Expect(err).ToNot(HaveOccured())

				num, err := protocol.Messages(&protocol.PingRequest{}).WriteTo(conn)
				Expect(err).ToNot(HaveOccured())
				Expect(num).ToNot(Equal(0))

				var response protocol.PingResponse
				err = messagereader.ReadMessage(conn, &response)
				Expect(err).ToNot(HaveOccured())

				close(done)
			}, 1.0)
		})

		Context("and the client sends a EchoRequest", func() {
			It("sends an EchoResponse with the same message", func(done Done) {
				conn, err := net.Dial("unix", socketPath)
				Expect(err).ToNot(HaveOccured())

				message := proto.String("Hello, world!")

				num, err := protocol.Messages(&protocol.EchoRequest{Message: message}).WriteTo(conn)
				Expect(err).ToNot(HaveOccured())
				Expect(num).ToNot(Equal(0))

				var response protocol.EchoResponse
				err = messagereader.ReadMessage(conn, &response)
				Expect(err).ToNot(HaveOccured())

				Expect(response.GetMessage()).To(Equal(*message))

				close(done)
			}, 1.0)
		})
	})
})

func ErrorDialingUnix(socketPath string) func() error {
	return func() error {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
		}

		return err
	}
}
