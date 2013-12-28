package server_test

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"time"

	"code.google.com/p/gogoprotobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/fake_backend"
	"github.com/vito/garden/message_reader"
	protocol "github.com/vito/garden/protocol"
	"github.com/vito/garden/server"
)

var _ = Describe("The Warden server", func() {
	It("listens on the given socket path and chmods it to 0777", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		snapshotsPath := path.Join(tmpdir, "snapshots")

		wardenServer := server.New(socketPath, snapshotsPath, fake_backend.New())

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())

		Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccurred())

		stat, err := os.Stat(socketPath)
		Expect(err).ToNot(HaveOccurred())

		Expect(int(stat.Mode() & 0777)).To(Equal(0777))
	})

	It("deletes the socket file if it is already there", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		snapshotsPath := path.Join(tmpdir, "snapshots")

		socket, err := os.Create(socketPath)
		Expect(err).ToNot(HaveOccurred())
		socket.WriteString("oops")
		socket.Close()

		wardenServer := server.New(socketPath, snapshotsPath, fake_backend.New())

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())
	})

	It("creates the snapshots directory if it's not already there", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		snapshotsPath := path.Join(tmpdir, "snapshots")

		wardenServer := server.New(socketPath, snapshotsPath, fake_backend.New())

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())

		stat, err := os.Stat(snapshotsPath)
		Expect(err).ToNot(HaveOccurred())

		Expect(stat.IsDir()).To(BeTrue())
	})

	Context("when the snapshots directory fails to be created", func() {
		It("fails to start", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			tmpfile, err := ioutil.TempFile(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			wardenServer := server.New(
				path.Join(tmpdir, "warden.sock"),
				// weird scenario: /foo/X/snapshots with X being a file
				path.Join(tmpfile.Name(), "snapshots"),
				fake_backend.New(),
			)

			err = wardenServer.Start()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when no snapshots directory is given", func() {
		It("successfully starts", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			wardenServer := server.New(
				path.Join(tmpdir, "warden.sock"),
				"",
				fake_backend.New(),
			)

			err = wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when starting fails", func() {
		It("returns the error", func() {
			tmpfile, err := ioutil.TempFile(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			wardenServer := server.New(
				// weird scenario: /foo/X/warden.sock with X being a file
				path.Join(tmpfile.Name(), "warden.sock"),
				path.Join(tmpfile.Name(), "snapshots"),
				fake_backend.New(),
			)

			err = wardenServer.Start()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("shutting down", func() {
		var socketPath string
		var snapshotsPath string

		var serverBackend backend.Backend
		var fakeBackend *fake_backend.FakeBackend

		var wardenServer *server.WardenServer

		var serverConnection net.Conn
		var responses *bufio.Reader

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			socketPath = path.Join(tmpdir, "warden.sock")
			snapshotsPath = path.Join(tmpdir, "snapshots")
			fakeBackend = fake_backend.New()

			serverBackend = fakeBackend
		})

		JustBeforeEach(func() {
			wardenServer = server.New(socketPath, snapshotsPath, serverBackend)

			err := wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())

			Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccurred())

			serverConnection, err = net.Dial("unix", socketPath)
			Expect(err).ToNot(HaveOccurred())

			responses = bufio.NewReader(serverConnection)
		})

		writeMessages := func(message proto.Message) {
			num, err := protocol.Messages(message).WriteTo(serverConnection)
			Expect(err).ToNot(HaveOccurred())
			Expect(num).ToNot(Equal(0))
		}

		readResponse := func(response proto.Message) {
			err := message_reader.ReadMessage(responses, response)
			Expect(err).ToNot(HaveOccurred())
		}

		It("stops accepting new connections", func() {
			go wardenServer.Stop()
			Eventually(ErrorDialingUnix(socketPath)).Should(HaveOccurred())
		})

		It("stops handling requests on existing connections", func() {
			writeMessages(&protocol.PingRequest{})
			readResponse(&protocol.PingResponse{})

			go wardenServer.Stop()

			// server was already reading a request
			_, err := protocol.Messages(&protocol.PingRequest{}).WriteTo(serverConnection)
			Expect(err).ToNot(HaveOccurred())

			// server will not actually handle it
			err = message_reader.ReadMessage(responses, &protocol.PingResponse{})
			Expect(err).To(HaveOccurred())
		})

		Context("when a Create request is in-flight", func() {
			BeforeEach(func() {
				serverBackend = fake_backend.NewSlow(1 * time.Second)
			})

			It("waits for it to complete and stops accepting requests", func() {
				writeMessages(&protocol.CreateRequest{})

				time.Sleep(100 * time.Millisecond)

				before := time.Now()

				wardenServer.Stop()

				Expect(time.Since(before)).To(BeNumerically(">", 500*time.Millisecond))

				readResponse(&protocol.CreateResponse{})

				_, err := protocol.Messages(&protocol.PingRequest{}).WriteTo(serverConnection)
				Expect(err).To(HaveOccurred())
			})
		})

		dontWaitRequests := []proto.Message{
			&protocol.LinkRequest{
				Handle: proto.String("some-handle"),
				JobId:  proto.Uint32(1),
			},
			&protocol.StreamRequest{
				Handle: proto.String("some-handle"),
				JobId:  proto.Uint32(1),
			},
			&protocol.RunRequest{
				Handle: proto.String("some-handle"),
				Script: proto.String("some-script"),
			},
		}

		for _, req := range dontWaitRequests {
			request := req

			Context(fmt.Sprintf("when a %T request is in-flight", request), func() {
				BeforeEach(func() {
					serverBackend = fake_backend.NewSlow(1 * time.Second)

					container, err := serverBackend.Create(backend.ContainerSpec{Handle: "some-handle"})
					Expect(err).ToNot(HaveOccurred())

					exitStatus := uint32(42)

					fakeContainer := container.(*fake_backend.FakeContainer)

					fakeContainer.StreamedJobChunks = []backend.JobStream{
						{
							ExitStatus: &exitStatus,
						},
					}
				})

				It("does not wait for it to complete", func() {
					writeMessages(request)

					time.Sleep(100 * time.Millisecond)

					before := time.Now()

					wardenServer.Stop()

					Expect(time.Since(before)).To(BeNumerically("<", 500*time.Millisecond))

					response := protocol.ResponseMessageForType(protocol.TypeForMessage(request))
					readResponse(response)

					_, err := protocol.Messages(&protocol.PingRequest{}).WriteTo(serverConnection)
					Expect(err).To(HaveOccurred())
				})
			})
		}
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

func uint64ptr(n uint64) *uint64 {
	return &n
}
