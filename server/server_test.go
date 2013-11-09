package server_test

import (
	"errors"
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
	It("listens on the given socket path", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccured())

		socketPath := path.Join(tmpdir, "warden.sock")

		wardenServer := server.New(socketPath, fake_backend.New())

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
				fake_backend.New(),
			)

			err = wardenServer.Start()
			Expect(err).To(HaveOccured())
		})
	})

	Context("when a client connects", func() {
		var socketPath string
		var serverBackend *fake_backend.FakeBackend

		var serverConnection net.Conn

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccured())

			socketPath = path.Join(tmpdir, "warden.sock")
			serverBackend = fake_backend.New()

			wardenServer := server.New(socketPath, serverBackend)

			err = wardenServer.Start()
			Expect(err).ToNot(HaveOccured())

			Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccured())

			serverConnection, err = net.Dial("unix", socketPath)
			Expect(err).ToNot(HaveOccured())
		})

		writeMessages := func(message proto.Message) {
			num, err := protocol.Messages(message).WriteTo(serverConnection)
			Expect(err).ToNot(HaveOccured())
			Expect(num).ToNot(Equal(0))
		}

		readResponse := func(response proto.Message) {
			err := message_reader.ReadMessage(serverConnection, response)
			Expect(err).ToNot(HaveOccured())
		}

		Context("and the client sends a PingRequest", func() {
			It("sends a PongResponse", func(done Done) {
				writeMessages(&protocol.PingRequest{})
				readResponse(&protocol.PingResponse{})
				close(done)
			}, 1.0)
		})

		Context("and the client sends a EchoRequest", func() {
			It("sends an EchoResponse with the same message", func(done Done) {
				message := proto.String("Hello, world!")

				writeMessages(&protocol.EchoRequest{Message: message})

				var response protocol.EchoResponse
				readResponse(&response)

				Expect(response.GetMessage()).To(Equal(*message))

				close(done)
			}, 1.0)
		})

		Context("and the client sends a CreateRequest", func() {
			It("sends a CreateResponse with the created handle", func(done Done) {
				writeMessages(&protocol.CreateRequest{
					Handle: proto.String("some-handle"),
				})

				var response protocol.CreateResponse
				readResponse(&response)

				Expect(response.GetHandle()).To(Equal("some-handle"))

				close(done)
			}, 1.0)

			It("creates the container with the spec from the request", func(done Done) {
				var bindMountMode protocol.CreateRequest_BindMount_Mode

				bindMountMode = protocol.CreateRequest_BindMount_RW

				writeMessages(&protocol.CreateRequest{
					Handle:    proto.String("some-handle"),
					GraceTime: proto.Uint32(42),
					Network:   proto.String("some-network"),
					Rootfs:    proto.String("/path/to/rootfs"),
					BindMounts: []*protocol.CreateRequest_BindMount{
						{
							SrcPath: proto.String("/bind/mount/src"),
							DstPath: proto.String("/bind/mount/dst"),
							Mode:    &bindMountMode,
						},
					},
				})

				var response protocol.CreateResponse
				readResponse(&response)

				container, found := serverBackend.CreatedContainers[response.GetHandle()]
				Expect(found).To(BeTrue())

				Expect(container.Spec).To(Equal(backend.ContainerSpec{
					Handle:     "some-handle",
					GraceTime:  time.Duration(42 * time.Second),
					Network:    "some-network",
					RootFSPath: "/path/to/rootfs",
					BindMounts: []backend.BindMount{
						{
							SrcPath: "/bind/mount/src",
							DstPath: "/bind/mount/dst",
							Mode:    backend.BindMountModeRW,
						},
					},
				}))

				close(done)
			}, 1.0)

			Context("when creating the container fails", func() {
				BeforeEach(func() {
					serverBackend.CreateError = errors.New("oh no!")
				})

				It("sends a WardenError response", func(done Done) {
					writeMessages(&protocol.CreateRequest{
						Handle: proto.String("some-handle"),
					})

					var response protocol.CreateResponse
					err := message_reader.ReadMessage(serverConnection, &response)
					Expect(err).To(Equal(&message_reader.WardenError{Message: "oh no!"}))

					close(done)
				}, 1.0)
			})
		})

		Context("and the client sends a DestroyRequest", func() {
			BeforeEach(func() {
				_, err := serverBackend.Create(backend.ContainerSpec{Handle: "some-handle"})
				Expect(err).ToNot(HaveOccured())
			})

			It("destroys the container and sends a DestroyResponse", func(done Done) {
				writeMessages(&protocol.DestroyRequest{
					Handle: proto.String("some-handle"),
				})

				var response protocol.DestroyResponse
				readResponse(&response)

				Expect(serverBackend.CreatedContainers).ToNot(HaveKey("some-handle"))

				close(done)
			}, 1.0)

			Context("when destroying the container fails", func() {
				BeforeEach(func() {
					serverBackend.DestroyError = errors.New("oh no!")
				})

				It("sends a WardenError response", func(done Done) {
					writeMessages(&protocol.DestroyRequest{
						Handle: proto.String("some-handle"),
					})

					var response protocol.DestroyResponse
					err := message_reader.ReadMessage(serverConnection, &response)
					Expect(err).To(Equal(&message_reader.WardenError{Message: "oh no!"}))

					close(done)
				}, 1.0)
			})
		})

		Context("and the client sends a ListRequest", func() {
			BeforeEach(func() {
				_, err := serverBackend.Create(backend.ContainerSpec{Handle: "some-handle"})
				Expect(err).ToNot(HaveOccured())

				_, err = serverBackend.Create(backend.ContainerSpec{Handle: "another-handle"})
				Expect(err).ToNot(HaveOccured())
			})

			It("sends a ListResponse containing the existing handles", func(done Done) {
				writeMessages(&protocol.ListRequest{})

				var response protocol.ListResponse
				readResponse(&response)

				Expect(response.GetHandles()).To(ContainElement("some-handle"))
				Expect(response.GetHandles()).To(ContainElement("another-handle"))

				close(done)
			}, 1.0)

			Context("when getting the containers fails", func() {
				BeforeEach(func() {
					serverBackend.ContainersError = errors.New("oh no!")
				})

				It("sends a WardenError response", func(done Done) {
					writeMessages(&protocol.ListRequest{})

					var response protocol.ListResponse
					err := message_reader.ReadMessage(serverConnection, &response)
					Expect(err).To(Equal(&message_reader.WardenError{Message: "oh no!"}))

					close(done)
				}, 1.0)
			})

			Context("and the client sends a CopyInRequest", func() {
				var fakeContainer *fake_backend.FakeContainer

				BeforeEach(func() {
					container, err := serverBackend.Create(backend.ContainerSpec{Handle: "some-handle"})
					Expect(err).ToNot(HaveOccured())

					fakeContainer = container.(*fake_backend.FakeContainer)
				})

				It("copies the file in and sends a CopyInResponse", func(done Done) {
					writeMessages(&protocol.CopyInRequest{
						Handle:  proto.String(fakeContainer.Handle()),
						SrcPath: proto.String("/src/path"),
						DstPath: proto.String("/dst/path"),
					})

					var response protocol.CopyInResponse
					readResponse(&response)

					Expect(fakeContainer.CopiedIn).To(ContainElement(
						[]string{"/src/path", "/dst/path"},
					))

					close(done)
				}, 1.0)

				Context("when the container is not found", func() {
					BeforeEach(func() {
						serverBackend.Destroy(fakeContainer.Handle())
					})

					It("sends a WardenError response", func(done Done) {
						writeMessages(&protocol.CopyInRequest{
							Handle:  proto.String(fakeContainer.Handle()),
							SrcPath: proto.String("/src/path"),
							DstPath: proto.String("/dst/path"),
						})

						var response protocol.CopyInResponse
						err := message_reader.ReadMessage(serverConnection, &response)
						Expect(err).To(Equal(&message_reader.WardenError{
							Message: "unknown handle: some-handle",
						}))

						close(done)
					}, 1.0)
				})

				Context("when copying in to the container fails", func() {
					BeforeEach(func() {
						fakeContainer.CopyInError = errors.New("oh no!")
					})

					It("sends a WardenError response", func(done Done) {
						writeMessages(&protocol.CopyInRequest{
							Handle:  proto.String(fakeContainer.Handle()),
							SrcPath: proto.String("/src/path"),
							DstPath: proto.String("/dst/path"),
						})

						var response protocol.CopyInResponse
						err := message_reader.ReadMessage(serverConnection, &response)
						Expect(err).To(Equal(&message_reader.WardenError{Message: "oh no!"}))

						close(done)
					}, 1.0)
				})
			})
		})

		Context("and the client sends a CopyOutRequest", func() {
			var fakeContainer *fake_backend.FakeContainer

			BeforeEach(func() {
				container, err := serverBackend.Create(backend.ContainerSpec{Handle: "some-handle"})
				Expect(err).ToNot(HaveOccured())

				fakeContainer = container.(*fake_backend.FakeContainer)
			})

			It("copies the file out and sends a CopyOutResponse", func(done Done) {
				writeMessages(&protocol.CopyOutRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					SrcPath: proto.String("/src/path"),
					DstPath: proto.String("/dst/path"),
					Owner:   proto.String("someuser"),
				})

				var response protocol.CopyOutResponse
				readResponse(&response)

				Expect(fakeContainer.CopiedOut).To(ContainElement(
					[]string{"/src/path", "/dst/path", "someuser"},
				))

				close(done)
			}, 1.0)

			Context("when the container is not found", func() {
				BeforeEach(func() {
					serverBackend.Destroy(fakeContainer.Handle())
				})

				It("sends a WardenError response", func(done Done) {
					writeMessages(&protocol.CopyOutRequest{
						Handle:  proto.String(fakeContainer.Handle()),
						SrcPath: proto.String("/src/path"),
						DstPath: proto.String("/dst/path"),
						Owner:   proto.String("someuser"),
					})

					var response protocol.CopyOutResponse
					err := message_reader.ReadMessage(serverConnection, &response)
					Expect(err).To(Equal(&message_reader.WardenError{
						Message: "unknown handle: some-handle",
					}))

					close(done)
				}, 1.0)
			})

			Context("when copying out of the container fails", func() {
				BeforeEach(func() {
					fakeContainer.CopyOutError = errors.New("oh no!")
				})

				It("sends a WardenError response", func(done Done) {
					writeMessages(&protocol.CopyOutRequest{
						Handle:  proto.String(fakeContainer.Handle()),
						SrcPath: proto.String("/src/path"),
						DstPath: proto.String("/dst/path"),
						Owner:   proto.String("someuser"),
					})

					var response protocol.CopyOutResponse
					err := message_reader.ReadMessage(serverConnection, &response)
					Expect(err).To(Equal(&message_reader.WardenError{Message: "oh no!"}))

					close(done)
				}, 1.0)
			})
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
