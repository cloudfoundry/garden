package server_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"time"

	"code.google.com/p/gogoprotobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/cloudfoundry-incubator/garden/transport"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/cloudfoundry-incubator/garden/warden/fake_backend"
)

var _ = Describe("When a client connects", func() {
	var socketPath string

	var serverBackend *fake_backend.FakeBackend

	var serverContainerGraceTime time.Duration

	var wardenServer *server.WardenServer

	var serverConnection net.Conn
	var responses *bufio.Reader

	BeforeEach(func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath = path.Join(tmpdir, "warden.sock")
		serverBackend = fake_backend.New()
		serverContainerGraceTime = 42 * time.Second

		wardenServer = server.New(
			"unix",
			socketPath,
			serverContainerGraceTime,
			serverBackend,
		)

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())

		Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())

		serverConnection, err = net.Dial("unix", socketPath)
		Expect(err).ToNot(HaveOccurred())

		responses = bufio.NewReader(serverConnection)
	})

	writeMessages := func(messages ...proto.Message) {
		num, err := protocol.Messages(messages...).WriteTo(serverConnection)
		Expect(err).ToNot(HaveOccurred())
		Expect(num).ToNot(Equal(0))
	}

	readResponse := func(response proto.Message) {
		err := transport.ReadMessage(responses, response)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}

	readAnyResponse := func() {
		err := transport.ReadMessage(responses, &protocol.CreateResponse{})
		if _, ok := err.(*transport.TypeMismatchError); !ok {
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		}
	}

	itResetsGraceTimeWhenHandling := func(request proto.Message) {
		Context(fmt.Sprintf("when handling a %T", request), func() {
			It("resets the container's grace time", func(done Done) {
				writeMessages(&protocol.CreateRequest{
					Handle:    proto.String("some-handle"),
					GraceTime: proto.Uint32(1),
				})

				var response protocol.CreateResponse
				readResponse(&response)

				for i := 0; i < 11; i++ {
					time.Sleep(100 * time.Millisecond)

					writeMessages(request)
					readAnyResponse()
				}

				before := time.Now()

				Eventually(func() error {
					_, err := serverBackend.Lookup("some-handle")
					return err
				}, 2.0).Should(HaveOccurred())

				Expect(time.Since(before)).To(BeNumerically(">", 1*time.Second))

				close(done)
			}, 5.0)
		})
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

	Context("and the client sends a CapacityRequest", func() {
		BeforeEach(func() {
			serverBackend.CapacityResult = warden.Capacity{
				MemoryInBytes: 1111,
				DiskInBytes:   2222,
				MaxContainers: 42,
			}
		})

		It("sends an CapacityResponse with the backend's reported capacity", func(done Done) {
			writeMessages(&protocol.CapacityRequest{})

			var response protocol.CapacityResponse
			readResponse(&response)

			Expect(response.GetMemoryInBytes()).To(Equal(uint64(1111)))
			Expect(response.GetDiskInBytes()).To(Equal(uint64(2222)))
			Expect(response.GetMaxContainers()).To(Equal(uint64(42)))

			close(done)
		}, 1.0)

		Context("when getting the capacity fails", func() {
			BeforeEach(func() {
				serverBackend.CapacityError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.CapacityRequest{})

				var response protocol.CapacityResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
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
			var bindMountOrigin protocol.CreateRequest_BindMount_Origin

			bindMountMode = protocol.CreateRequest_BindMount_RW
			bindMountOrigin = protocol.CreateRequest_BindMount_Container

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
						Origin:  &bindMountOrigin,
					},
				},
				Properties: []*protocol.Property{
					{
						Key:   proto.String("cf-owner"),
						Value: proto.String("executor"),
					},
					{
						Key:   proto.String("duped"),
						Value: proto.String("a"),
					},
					{
						Key:   proto.String("duped"),
						Value: proto.String("b"),
					},
				},
			})

			var response protocol.CreateResponse
			readResponse(&response)

			container, found := serverBackend.CreatedContainers[response.GetHandle()]
			Expect(found).To(BeTrue())

			Expect(container.Spec).To(Equal(warden.ContainerSpec{
				Handle:     "some-handle",
				GraceTime:  time.Duration(42 * time.Second),
				Network:    "some-network",
				RootFSPath: "/path/to/rootfs",
				BindMounts: []warden.BindMount{
					{
						SrcPath: "/bind/mount/src",
						DstPath: "/bind/mount/dst",
						Mode:    warden.BindMountModeRW,
						Origin:  warden.BindMountOriginContainer,
					},
				},
				Properties: map[string]string{
					"cf-owner": "executor",
					"duped":    "b",
				},
			}))

			close(done)
		}, 1.0)

		Context("when a grace time is given", func() {
			It("destroys the container after it has been idle for the grace time", func(done Done) {
				before := time.Now()

				writeMessages(&protocol.CreateRequest{
					Handle:    proto.String("some-handle"),
					GraceTime: proto.Uint32(1),
				})

				var response protocol.CreateResponse
				readResponse(&response)

				_, err := serverBackend.Lookup("some-handle")
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() error {
					_, err := serverBackend.Lookup("some-handle")
					return err
				}, 2.0).Should(HaveOccurred())

				Expect(time.Since(before)).To(BeNumerically(">", 1*time.Second))

				close(done)
			}, 5.0)
		})

		Context("when a grace time is not given", func() {
			It("defaults it to the server's grace time", func(done Done) {
				writeMessages(&protocol.CreateRequest{
					Handle: proto.String("some-handle"),
				})

				var response protocol.CreateResponse
				readResponse(&response)

				container, err := serverBackend.Lookup("some-handle")
				Expect(err).ToNot(HaveOccurred())

				Expect(serverBackend.GraceTime(container)).To(Equal(serverContainerGraceTime))

				close(done)
			}, 1.0)
		})

		Context("when creating the container fails", func() {
			BeforeEach(func() {
				serverBackend.CreateError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.CreateRequest{
					Handle: proto.String("some-handle"),
				})

				var response protocol.CreateResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a DestroyRequest", func() {
		BeforeEach(func() {
			_, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())
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
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		It("removes the grace timer", func(done Done) {
			writeMessages(&protocol.CreateRequest{
				Handle:    proto.String("some-other-handle"),
				GraceTime: proto.Uint32(1),
			})

			var response protocol.CreateResponse
			readResponse(&response)

			writeMessages(&protocol.DestroyRequest{
				Handle: proto.String("some-other-handle"),
			})

			var destroyResponse protocol.DestroyResponse
			readResponse(&destroyResponse)

			time.Sleep(2 * time.Second)

			Expect(serverBackend.DestroyedContainers).To(HaveLen(1))

			close(done)
		}, 5.0)
	})

	Context("and the client sends a ListRequest", func() {
		BeforeEach(func() {
			_, err := serverBackend.Create(warden.ContainerSpec{
				Handle: "some-handle",
				Properties: map[string]string{
					"cf-owner": "executor",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = serverBackend.Create(warden.ContainerSpec{
				Handle: "another-handle",
				Properties: map[string]string{
					"cf-owner": "executor",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = serverBackend.Create(warden.ContainerSpec{
				Handle: "super-handle",
				Properties: map[string]string{
					"cf-owner": "pants",
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("sends a ListResponse containing the existing handles", func(done Done) {
			writeMessages(&protocol.ListRequest{})

			var response protocol.ListResponse
			readResponse(&response)

			Expect(response.GetHandles()).To(ContainElement("some-handle"))
			Expect(response.GetHandles()).To(ContainElement("another-handle"))
			Expect(response.GetHandles()).To(ContainElement("super-handle"))

			close(done)
		}, 1.0)

		Context("when getting the containers fails", func() {
			BeforeEach(func() {
				serverBackend.ContainersError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.ListRequest{})

				var response protocol.ListResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		Context("and the client sends a ListRequest with a property filter", func() {
			It("forwards the filter to the backend", func(done Done) {
				writeMessages(&protocol.ListRequest{
					Properties: []*protocol.Property{
						{
							Key:   proto.String("foo"),
							Value: proto.String("bar"),
						},
					},
				})

				var response protocol.ListResponse
				readResponse(&response)

				Expect(serverBackend.ContainersFilters).To(ContainElement(
					warden.Properties{
						"foo": "bar",
					},
				))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a StopRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("stops the container and sends a StopResponse", func(done Done) {
			writeMessages(&protocol.StopRequest{
				Handle: proto.String(fakeContainer.Handle()),
				Kill:   proto.Bool(true),
			})

			var response protocol.StopResponse
			readResponse(&response)

			Expect(fakeContainer.Stopped()).To(ContainElement(
				fake_backend.StopSpec{
					Killed: true,
				},
			))

			close(done)
		}, 1.0)

		Context("when background is true", func() {
			It("stops async and returns immediately", func(done Done) {
				fakeContainer.StopCallback = func() {
					time.Sleep(100 * time.Millisecond)
				}

				writeMessages(&protocol.StopRequest{
					Handle:     proto.String(fakeContainer.Handle()),
					Kill:       proto.Bool(true),
					Background: proto.Bool(true),
				})

				var response protocol.StopResponse
				readResponse(&response)

				Expect(fakeContainer.Stopped()).To(BeEmpty())
				Eventually(fakeContainer.Stopped).ShouldNot(BeEmpty())

				close(done)
			}, 1.0)
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.StopRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.StopResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when stopping the container fails", func() {
			BeforeEach(func() {
				fakeContainer.StopError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.StopRequest{
					Handle: proto.String("some-handle"),
				})

				var response protocol.StopResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		itResetsGraceTimeWhenHandling(
			&protocol.StopRequest{
				Handle: proto.String("some-handle"),
			},
		)
	})

	Context("and the client sends a StreamInRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		startStream := func() {
			writeMessages(
				&protocol.StreamInRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					DstPath: proto.String("/dst/path"),
				},
			)
		}

		sendChunks := func() {
			writeMessages(
				&protocol.StreamChunk{
					Content: []byte("chunk-1;"),
				},
				&protocol.StreamChunk{
					Content: []byte("chunk-2;"),
				},
				&protocol.StreamChunk{
					Content: []byte("chunk-3;"),
				},
				&protocol.StreamChunk{
					EOF: proto.Bool(true),
				},
			)
		}

		It("sends an initial StreamInResponse, streams the file in, waits for completion, and sends a StreamInResponse", func(done Done) {
			startStream()

			var response protocol.StreamInResponse
			readResponse(&response)

			Expect(fakeContainer.StreamedIn).To(HaveLen(1))

			streamedIn := fakeContainer.StreamedIn[0]
			Expect(streamedIn.DestPath).To(Equal("/dst/path"))

			sendChunks()

			Eventually(streamedIn.InStream).Should(gbytes.Say("chunk-1;chunk-2;chunk-3;"))

			Expect(streamedIn.InStream.Closed()).Should(BeTrue())

			readResponse(&response)

			close(done)
		}, 5.0)

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				startStream()

				err := transport.ReadMessage(responses, &protocol.StreamInResponse{})
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when copying in to the container fails", func() {
			BeforeEach(func() {
				fakeContainer.StreamInError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				startStream()

				err := transport.ReadMessage(responses, &protocol.StreamInResponse{})
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a StreamOutRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
			fakeContainer.StreamOutBuffer = bytes.NewBuffer([]byte("hello-world!"))
		})

		It("streams the file out and sends a StreamOutResponse", func(done Done) {
			writeMessages(&protocol.StreamOutRequest{
				Handle:  proto.String(fakeContainer.Handle()),
				SrcPath: proto.String("/src/path"),
			})

			var response protocol.StreamOutResponse
			readResponse(&response)

			streamedContent := ""
			for {
				var chunk protocol.StreamChunk
				readResponse(&chunk)
				if chunk.GetEOF() {
					break
				}
				streamedContent += string(chunk.Content)
			}

			Expect(streamedContent).To(Equal("hello-world!"))

			Expect(fakeContainer.StreamedOut).To(Equal([]string{
				"/src/path",
			}))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.StreamOutRequest{
			Handle:  proto.String("some-handle"),
			SrcPath: proto.String("/src/path"),
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.StreamOutRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					SrcPath: proto.String("/src/path"),
				})

				err := transport.ReadMessage(responses, &protocol.StreamOutResponse{})
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when streaming out of the container fails", func() {
			BeforeEach(func() {
				fakeContainer.StreamOutError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.StreamOutRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					SrcPath: proto.String("/src/path"),
				})

				transport.ReadMessage(responses, &protocol.StreamOutResponse{})
				err := transport.ReadMessage(responses, &protocol.StreamOutResponse{})
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends an AttachRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("responds with a ProcessPayload for every chunk", func(done Done) {
			exitStatus := uint32(42)

			fakeContainer.StreamedProcessChunks = []warden.ProcessStream{
				{
					Source:     warden.ProcessStreamSourceStdout,
					Data:       []byte("process out\n"),
					ExitStatus: nil,
				},
				{
					Source:     warden.ProcessStreamSourceStderr,
					Data:       []byte("process err\n"),
					ExitStatus: nil,
				},
				{
					ExitStatus: &exitStatus,
				},
			}

			writeMessages(&protocol.AttachRequest{
				Handle:    proto.String(fakeContainer.Handle()),
				ProcessId: proto.Uint32(123),
			})

			var response protocol.ProcessPayload
			readResponse(&response)

			Expect(response.GetProcessId()).To(Equal(uint32(123)))
			Expect(response.GetSource()).To(Equal(protocol.ProcessPayload_stdout))
			Expect(response.GetData()).To(Equal("process out\n"))
			Expect(response.ExitStatus).To(BeNil())

			readResponse(&response)

			Expect(response.GetSource()).To(Equal(protocol.ProcessPayload_stderr))
			Expect(response.GetData()).To(Equal("process err\n"))
			Expect(response.ExitStatus).To(BeNil())

			readResponse(&response)

			Expect(response.GetSource()).To(BeZero())
			Expect(response.GetData()).To(BeZero())
			Expect(response.GetExitStatus()).To(Equal(uint32(42)))

			Expect(fakeContainer.Attached).To(ContainElement(uint32(123)))

			close(done)
		}, 1.0)

		It("resets the container's grace time as long as it's streaming", func(done Done) {
			writeMessages(&protocol.CreateRequest{
				Handle:    proto.String("some-handle"),
				GraceTime: proto.Uint32(1),
			})

			var response protocol.CreateResponse
			readResponse(&response)

			fakeContainer := serverBackend.CreatedContainers["some-handle"]

			exitStatus := uint32(42)

			fakeContainer.StreamedProcessChunks = []warden.ProcessStream{
				{
					Source:     warden.ProcessStreamSourceStdout,
					Data:       []byte("process out\n"),
					ExitStatus: nil,
				},
				{
					Source:     warden.ProcessStreamSourceStderr,
					Data:       []byte("process err\n"),
					ExitStatus: nil,
				},
				{
					ExitStatus: &exitStatus,
				},
			}

			fakeContainer.StreamDelay = 1 * time.Second

			writeMessages(&protocol.AttachRequest{
				Handle:    proto.String(fakeContainer.Handle()),
				ProcessId: proto.Uint32(123),
			})

			var payloadResponse protocol.ProcessPayload
			readResponse(&payloadResponse)
			readResponse(&payloadResponse)
			readResponse(&payloadResponse)

			before := time.Now()

			Eventually(func() error {
				_, err := serverBackend.Lookup("some-handle")
				return err
			}, 2.0).Should(HaveOccurred())

			Expect(time.Since(before)).To(BeNumerically(">", 1*time.Second))

			close(done)
		}, 5.0)

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.AttachRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					ProcessId: proto.Uint32(123),
				})

				var response protocol.ProcessPayload

				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when streaming fails", func() {
			BeforeEach(func() {
				fakeContainer.AttachError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.AttachRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					ProcessId: proto.Uint32(123),
				})

				var response protocol.ProcessPayload

				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a RunRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("runs the process and streams the output", func(done Done) {
			fakeContainer.RunningProcessID = 123
			exitStatus := uint32(42)

			fakeContainer.StreamedProcessChunks = []warden.ProcessStream{
				{
					Source:     warden.ProcessStreamSourceStdout,
					Data:       []byte("process out\n"),
					ExitStatus: nil,
				},
				{
					Source:     warden.ProcessStreamSourceStderr,
					Data:       []byte("process err\n"),
					ExitStatus: nil,
				},
				{
					ExitStatus: &exitStatus,
				},
			}

			writeMessages(&protocol.RunRequest{
				Handle:     proto.String(fakeContainer.Handle()),
				Script:     proto.String("/some/script"),
				Privileged: proto.Bool(true),
				Rlimits: &protocol.ResourceLimits{
					As:         proto.Uint64(1),
					Core:       proto.Uint64(2),
					Cpu:        proto.Uint64(3),
					Data:       proto.Uint64(4),
					Fsize:      proto.Uint64(5),
					Locks:      proto.Uint64(6),
					Memlock:    proto.Uint64(7),
					Msgqueue:   proto.Uint64(8),
					Nice:       proto.Uint64(9),
					Nofile:     proto.Uint64(10),
					Nproc:      proto.Uint64(11),
					Rss:        proto.Uint64(12),
					Rtprio:     proto.Uint64(13),
					Sigpending: proto.Uint64(14),
					Stack:      proto.Uint64(15),
				},
				Env: []*protocol.EnvironmentVariable{
					&protocol.EnvironmentVariable{
						Key:   proto.String("FLAVOR"),
						Value: proto.String("chocolate"),
					},
					&protocol.EnvironmentVariable{
						Key:   proto.String("TOPPINGS"),
						Value: proto.String("sprinkles"),
					},
				},
			})

			var response protocol.ProcessPayload

			readResponse(&response)
			Expect(response.GetProcessId()).To(Equal(uint32(123)))
			Expect(response.GetSource()).To(BeZero())
			Expect(response.GetData()).To(BeZero())
			Expect(response.ExitStatus).To(BeNil())

			readResponse(&response)
			Expect(response.GetProcessId()).To(Equal(uint32(123)))
			Expect(response.GetSource()).To(Equal(protocol.ProcessPayload_stdout))
			Expect(response.GetData()).To(Equal("process out\n"))
			Expect(response.ExitStatus).To(BeNil())

			readResponse(&response)
			Expect(response.GetProcessId()).To(Equal(uint32(123)))
			Expect(response.GetSource()).To(Equal(protocol.ProcessPayload_stderr))
			Expect(response.GetData()).To(Equal("process err\n"))
			Expect(response.ExitStatus).To(BeNil())

			readResponse(&response)
			Expect(response.GetProcessId()).To(Equal(uint32(123)))
			Expect(response.GetSource()).To(BeZero())
			Expect(response.GetData()).To(BeZero())
			Expect(response.GetExitStatus()).To(Equal(uint32(42)))

			Expect(fakeContainer.RunningProcesses).To(ContainElement(
				warden.ProcessSpec{
					Script:     "/some/script",
					Privileged: true,
					Limits: warden.ResourceLimits{
						As:         uint64ptr(1),
						Core:       uint64ptr(2),
						Cpu:        uint64ptr(3),
						Data:       uint64ptr(4),
						Fsize:      uint64ptr(5),
						Locks:      uint64ptr(6),
						Memlock:    uint64ptr(7),
						Msgqueue:   uint64ptr(8),
						Nice:       uint64ptr(9),
						Nofile:     uint64ptr(10),
						Nproc:      uint64ptr(11),
						Rss:        uint64ptr(12),
						Rtprio:     uint64ptr(13),
						Sigpending: uint64ptr(14),
						Stack:      uint64ptr(15),
					},
					EnvironmentVariables: []warden.EnvironmentVariable{
						warden.EnvironmentVariable{
							Key:   "FLAVOR",
							Value: "chocolate",
						},
						warden.EnvironmentVariable{
							Key:   "TOPPINGS",
							Value: "sprinkles",
						},
					},
				},
			))

			close(done)
		}, 1.0)

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.RunRequest{
					Handle: proto.String(fakeContainer.Handle()),
					Script: proto.String("/some/script"),
				})

				var response protocol.ProcessPayload

				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when running fails", func() {
			BeforeEach(func() {
				fakeContainer.RunError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.RunRequest{
					Handle: proto.String(fakeContainer.Handle()),
					Script: proto.String("/some/script"),
				})

				var response protocol.ProcessPayload

				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		It("resets the container's grace time as long as it's streaming", func(done Done) {
			writeMessages(&protocol.CreateRequest{
				Handle:    proto.String("some-handle"),
				GraceTime: proto.Uint32(1),
			})

			var response protocol.CreateResponse
			readResponse(&response)

			fakeContainer := serverBackend.CreatedContainers["some-handle"]

			exitStatus := uint32(42)

			fakeContainer.StreamedProcessChunks = []warden.ProcessStream{
				{
					Source:     warden.ProcessStreamSourceStdout,
					Data:       []byte("process out\n"),
					ExitStatus: nil,
				},
				{
					Source:     warden.ProcessStreamSourceStderr,
					Data:       []byte("process err\n"),
					ExitStatus: nil,
				},
				{
					ExitStatus: &exitStatus,
				},
			}

			fakeContainer.StreamDelay = 1 * time.Second

			writeMessages(&protocol.RunRequest{
				Handle: proto.String("some-handle"),
				Script: proto.String("/some/script"),
			})

			var payloadResponse protocol.ProcessPayload
			readResponse(&payloadResponse) // read process id
			readResponse(&payloadResponse) // read stdout
			readResponse(&payloadResponse) // read stderr
			readResponse(&payloadResponse) // read exit status

			before := time.Now()

			Eventually(func() error {
				_, err := serverBackend.Lookup("some-handle")
				return err
			}, 2.0).Should(HaveOccurred())

			Expect(time.Since(before)).To(BeNumerically(">", 1*time.Second))

			close(done)
		}, 5.0)
	})

	Context("and the client sends a LimitBandwidthRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("sets the container's bandwidth limits and returns them", func(done Done) {
			setLimits := warden.BandwidthLimits{
				RateInBytesPerSecond:      123,
				BurstRateInBytesPerSecond: 456,
			}

			effectiveLimits := warden.BandwidthLimits{
				RateInBytesPerSecond:      1230,
				BurstRateInBytesPerSecond: 4560,
			}

			fakeContainer.CurrentBandwidthLimitsResult = effectiveLimits

			writeMessages(&protocol.LimitBandwidthRequest{
				Handle: proto.String(fakeContainer.Handle()),
				Rate:   proto.Uint64(setLimits.RateInBytesPerSecond),
				Burst:  proto.Uint64(setLimits.BurstRateInBytesPerSecond),
			})

			var response protocol.LimitBandwidthResponse
			readResponse(&response)

			Expect(fakeContainer.LimitedBandwidth).To(Equal(setLimits))

			Expect(response.GetRate()).To(Equal(effectiveLimits.RateInBytesPerSecond))
			Expect(response.GetBurst()).To(Equal(effectiveLimits.BurstRateInBytesPerSecond))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.LimitBandwidthRequest{
			Handle: proto.String("some-handle"),
			Rate:   proto.Uint64(123),
			Burst:  proto.Uint64(456),
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitBandwidthRequest{
					Handle: proto.String(fakeContainer.Handle()),
					Rate:   proto.Uint64(123),
					Burst:  proto.Uint64(456),
				})

				var response protocol.LimitBandwidthResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when limiting the bandwidth fails", func() {
			BeforeEach(func() {
				fakeContainer.LimitBandwidthError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitBandwidthRequest{
					Handle: proto.String(fakeContainer.Handle()),
					Rate:   proto.Uint64(123),
					Burst:  proto.Uint64(456),
				})

				var response protocol.LimitBandwidthResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		Context("when getting the current limits fails", func() {
			BeforeEach(func() {
				fakeContainer.CurrentBandwidthLimitsError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitBandwidthRequest{
					Handle: proto.String(fakeContainer.Handle()),
					Rate:   proto.Uint64(123),
					Burst:  proto.Uint64(456),
				})

				var response protocol.LimitBandwidthResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a LimitMemoryRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("sets the container's memory limits and returns the current limits", func(done Done) {
			setLimits := warden.MemoryLimits{1024}
			effectiveLimits := warden.MemoryLimits{2048}

			fakeContainer.CurrentMemoryLimitsResult = effectiveLimits

			writeMessages(&protocol.LimitMemoryRequest{
				Handle:       proto.String(fakeContainer.Handle()),
				LimitInBytes: proto.Uint64(setLimits.LimitInBytes),
			})

			var response protocol.LimitMemoryResponse
			readResponse(&response)

			Expect(fakeContainer.LimitedMemory).To(Equal(setLimits))

			Expect(response.GetLimitInBytes()).To(Equal(effectiveLimits.LimitInBytes))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.LimitMemoryRequest{
			Handle:       proto.String("some-handle"),
			LimitInBytes: proto.Uint64(123),
		})

		Context("when no limit is given", func() {
			It("does not change the memory limit", func(done Done) {
				effectiveLimits := warden.MemoryLimits{456}

				fakeContainer.CurrentMemoryLimitsResult = effectiveLimits

				writeMessages(&protocol.LimitMemoryRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.LimitMemoryResponse
				readResponse(&response)

				Expect(fakeContainer.DidLimitMemory).To(BeFalse())

				Expect(response.GetLimitInBytes()).To(Equal(effectiveLimits.LimitInBytes))

				close(done)
			})
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitMemoryRequest{
					Handle:       proto.String(fakeContainer.Handle()),
					LimitInBytes: proto.Uint64(123),
				})

				var response protocol.LimitMemoryResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when limiting the memory fails", func() {
			BeforeEach(func() {
				fakeContainer.LimitMemoryError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitMemoryRequest{
					Handle:       proto.String(fakeContainer.Handle()),
					LimitInBytes: proto.Uint64(123),
				})

				var response protocol.LimitMemoryResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		Context("when getting the current memory limits fails", func() {
			BeforeEach(func() {
				fakeContainer.CurrentMemoryLimitsError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitMemoryRequest{
					Handle:       proto.String(fakeContainer.Handle()),
					LimitInBytes: proto.Uint64(123),
				})

				var response protocol.LimitMemoryResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a LimitDiskRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("sets the container's disk limits and returns the current limits", func(done Done) {
			fakeContainer.CurrentDiskLimitsResult = warden.DiskLimits{
				BlockSoft: 1111,
				BlockHard: 2222,

				InodeSoft: 3333,
				InodeHard: 4444,

				ByteSoft: 5555,
				ByteHard: 6666,
			}

			writeMessages(&protocol.LimitDiskRequest{
				Handle:    proto.String(fakeContainer.Handle()),
				BlockSoft: proto.Uint64(111),
				BlockHard: proto.Uint64(222),
				InodeSoft: proto.Uint64(333),
				InodeHard: proto.Uint64(444),
				ByteSoft:  proto.Uint64(555),
				ByteHard:  proto.Uint64(666),
			})

			var response protocol.LimitDiskResponse
			readResponse(&response)

			Expect(fakeContainer.LimitedDisk).To(Equal(
				warden.DiskLimits{
					BlockSoft: 111,
					BlockHard: 222,

					InodeSoft: 333,
					InodeHard: 444,

					ByteSoft: 555,
					ByteHard: 666,
				},
			))

			Expect(response.GetBlockSoft()).To(Equal(uint64(1111)))
			Expect(response.GetBlockHard()).To(Equal(uint64(2222)))

			Expect(response.GetInodeSoft()).To(Equal(uint64(3333)))
			Expect(response.GetInodeHard()).To(Equal(uint64(4444)))

			Expect(response.GetByteSoft()).To(Equal(uint64(5555)))
			Expect(response.GetByteHard()).To(Equal(uint64(6666)))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.LimitDiskRequest{
			Handle:    proto.String("some-handle"),
			BlockSoft: proto.Uint64(111),
			Block:     proto.Uint64(222),
			InodeSoft: proto.Uint64(333),
			InodeHard: proto.Uint64(444),
		})

		Context("when no limits are given", func() {
			It("does not change the disk limit", func(done Done) {
				fakeContainer.CurrentDiskLimitsResult = warden.DiskLimits{
					BlockSoft: 1111,
					BlockHard: 2222,

					InodeSoft: 3333,
					InodeHard: 4444,

					ByteSoft: 5555,
					ByteHard: 6666,
				}

				writeMessages(&protocol.LimitDiskRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.DidLimitDisk).To(BeFalse())

				Expect(response.GetBlockSoft()).To(Equal(uint64(1111)))
				Expect(response.GetBlockHard()).To(Equal(uint64(2222)))

				Expect(response.GetInodeSoft()).To(Equal(uint64(3333)))
				Expect(response.GetInodeHard()).To(Equal(uint64(4444)))

				Expect(response.GetByteSoft()).To(Equal(uint64(5555)))
				Expect(response.GetByteHard()).To(Equal(uint64(6666)))

				close(done)
			})
		})

		Context("when Block is given", func() {
			It("passes it as BlockHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					Block:     proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,
					},
				))
			})
		})

		Context("when BlockLimit is given", func() {
			It("passes it as BlockHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:     proto.String(fakeContainer.Handle()),
					BlockSoft:  proto.Uint64(111),
					BlockLimit: proto.Uint64(222),
					InodeSoft:  proto.Uint64(333),
					InodeHard:  proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,
					},
				))
			})
		})

		Context("when Inode is given", func() {
			It("passes it as InodeHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					Inode:     proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,
					},
				))
			})
		})

		Context("when InodeLimit is given", func() {
			It("passes it as InodeHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:     proto.String(fakeContainer.Handle()),
					BlockSoft:  proto.Uint64(111),
					BlockHard:  proto.Uint64(222),
					InodeSoft:  proto.Uint64(333),
					InodeLimit: proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,
					},
				))
			})
		})

		Context("when Byte is given", func() {
			It("passes it as ByteHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
					Byte:      proto.Uint64(555),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,

						ByteHard: 555,
					},
				))
			})
		})

		Context("when ByteLimit is given", func() {
			It("passes it as ByteHard", func() {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
					ByteLimit: proto.Uint64(555),
				})

				var response protocol.LimitDiskResponse
				readResponse(&response)

				Expect(fakeContainer.LimitedDisk).To(Equal(
					warden.DiskLimits{
						BlockSoft: 111,
						BlockHard: 222,

						InodeSoft: 333,
						InodeHard: 444,

						ByteHard: 555,
					},
				))
			})
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when limiting the disk fails", func() {
			BeforeEach(func() {
				fakeContainer.LimitDiskError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		Context("when getting the current disk limits fails", func() {
			BeforeEach(func() {
				fakeContainer.CurrentDiskLimitsError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitDiskRequest{
					Handle:    proto.String(fakeContainer.Handle()),
					BlockSoft: proto.Uint64(111),
					BlockHard: proto.Uint64(222),
					InodeSoft: proto.Uint64(333),
					InodeHard: proto.Uint64(444),
				})

				var response protocol.LimitDiskResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a LimitCpuRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("sets the container's CPU shares and returns the current limits", func(done Done) {
			setLimits := warden.CPULimits{123}
			effectiveLimits := warden.CPULimits{456}

			fakeContainer.CurrentCPULimitsResult = effectiveLimits

			writeMessages(&protocol.LimitCpuRequest{
				Handle:        proto.String(fakeContainer.Handle()),
				LimitInShares: proto.Uint64(setLimits.LimitInShares),
			})

			var response protocol.LimitCpuResponse
			readResponse(&response)

			Expect(fakeContainer.LimitedCPU).To(Equal(setLimits))

			Expect(response.GetLimitInShares()).To(Equal(effectiveLimits.LimitInShares))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.LimitCpuRequest{
			Handle:        proto.String("some-handle"),
			LimitInShares: proto.Uint64(123),
		})

		Context("when no limit is given", func() {
			It("does not change the CPU shares", func(done Done) {
				effectiveLimits := warden.CPULimits{456}

				fakeContainer.CurrentCPULimitsResult = effectiveLimits

				writeMessages(&protocol.LimitCpuRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.LimitCpuResponse
				readResponse(&response)

				Expect(fakeContainer.DidLimitCPU).To(BeFalse())

				Expect(response.GetLimitInShares()).To(Equal(effectiveLimits.LimitInShares))

				close(done)
			})
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitCpuRequest{
					Handle:        proto.String(fakeContainer.Handle()),
					LimitInShares: proto.Uint64(123),
				})

				var response protocol.LimitCpuResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when limiting the CPU fails", func() {
			BeforeEach(func() {
				fakeContainer.LimitCPUError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitCpuRequest{
					Handle:        proto.String(fakeContainer.Handle()),
					LimitInShares: proto.Uint64(123),
				})

				var response protocol.LimitCpuResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})

		Context("when getting the current CPU limits fails", func() {
			BeforeEach(func() {
				fakeContainer.CurrentCPULimitsError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.LimitCpuRequest{
					Handle:        proto.String(fakeContainer.Handle()),
					LimitInShares: proto.Uint64(123),
				})

				var response protocol.LimitCpuResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a NetInRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("maps the ports and returns them", func(done Done) {
			writeMessages(&protocol.NetInRequest{
				Handle:        proto.String(fakeContainer.Handle()),
				HostPort:      proto.Uint32(123),
				ContainerPort: proto.Uint32(456),
			})

			var response protocol.NetInResponse
			readResponse(&response)

			Expect(fakeContainer.MappedIn).To(ContainElement(
				[]uint32{123, 456},
			))

			Expect(response.GetHostPort()).To(Equal(uint32(123)))
			Expect(response.GetContainerPort()).To(Equal(uint32(456)))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.NetInRequest{
			Handle:        proto.String("some-handle"),
			HostPort:      proto.Uint32(123),
			ContainerPort: proto.Uint32(456),
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.NetInRequest{
					Handle:        proto.String(fakeContainer.Handle()),
					HostPort:      proto.Uint32(123),
					ContainerPort: proto.Uint32(456),
				})

				var response protocol.NetInResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when mapping the port fails", func() {
			BeforeEach(func() {
				fakeContainer.NetInError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.NetInRequest{
					Handle:        proto.String(fakeContainer.Handle()),
					HostPort:      proto.Uint32(123),
					ContainerPort: proto.Uint32(456),
				})

				var response protocol.NetInResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a NetOutRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{Handle: "some-handle"})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("permits traffic outside of the container", func(done Done) {
			writeMessages(&protocol.NetOutRequest{
				Handle:  proto.String(fakeContainer.Handle()),
				Network: proto.String("1.2.3.4/22"),
				Port:    proto.Uint32(456),
			})

			var response protocol.NetOutResponse
			readResponse(&response)

			Expect(fakeContainer.PermittedOut).To(ContainElement(
				fake_backend.NetOutSpec{"1.2.3.4/22", 456},
			))

			close(done)
		}, 1.0)

		itResetsGraceTimeWhenHandling(&protocol.NetOutRequest{
			Handle:  proto.String("some-handle"),
			Network: proto.String("1.2.3.4/22"),
			Port:    proto.Uint32(456),
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.NetOutRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					Network: proto.String("1.2.3.4/22"),
					Port:    proto.Uint32(456),
				})

				var response protocol.NetOutResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when permitting traffic fails", func() {
			BeforeEach(func() {
				fakeContainer.NetOutError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.NetOutRequest{
					Handle:  proto.String(fakeContainer.Handle()),
					Network: proto.String("1.2.3.4/22"),
					Port:    proto.Uint32(456),
				})

				var response protocol.NetOutResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})

	Context("and the client sends a InfoRequest", func() {
		var fakeContainer *fake_backend.FakeContainer

		BeforeEach(func() {
			container, err := serverBackend.Create(warden.ContainerSpec{
				Handle: "some-handle",
				Properties: map[string]string{
					"foo": "bar",
					"a":   "b",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			fakeContainer = container.(*fake_backend.FakeContainer)
		})

		It("reports information about the container", func(done Done) {
			fakeContainer.ReportedInfo = warden.ContainerInfo{
				State:         "active",
				Events:        []string{"oom", "party"},
				HostIP:        "host-ip",
				ContainerIP:   "container-ip",
				ContainerPath: "/path/to/container",
				ProcessIDs:    []uint32{1, 2},
				MemoryStat: warden.ContainerMemoryStat{
					Cache:                   1,
					Rss:                     2,
					MappedFile:              3,
					Pgpgin:                  4,
					Pgpgout:                 5,
					Swap:                    6,
					Pgfault:                 7,
					Pgmajfault:              8,
					InactiveAnon:            9,
					ActiveAnon:              10,
					InactiveFile:            11,
					ActiveFile:              12,
					Unevictable:             13,
					HierarchicalMemoryLimit: 14,
					HierarchicalMemswLimit:  15,
					TotalCache:              16,
					TotalRss:                17,
					TotalMappedFile:         18,
					TotalPgpgin:             19,
					TotalPgpgout:            20,
					TotalSwap:               21,
					TotalPgfault:            22,
					TotalPgmajfault:         23,
					TotalInactiveAnon:       24,
					TotalActiveAnon:         25,
					TotalInactiveFile:       26,
					TotalActiveFile:         27,
					TotalUnevictable:        28,
				},
				CPUStat: warden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},
				DiskStat: warden.ContainerDiskStat{
					BytesUsed:  1,
					InodesUsed: 2,
				},
				BandwidthStat: warden.ContainerBandwidthStat{
					InRate:   1,
					InBurst:  2,
					OutRate:  3,
					OutBurst: 4,
				},
				MappedPorts: []warden.PortMapping{
					{HostPort: 1234, ContainerPort: 5678},
					{HostPort: 1235, ContainerPort: 5679},
				},
			}

			writeMessages(&protocol.InfoRequest{
				Handle: proto.String(fakeContainer.Handle()),
			})

			var response protocol.InfoResponse
			readResponse(&response)

			Expect(response.GetState()).To(Equal("active"))
			Expect(response.GetEvents()).To(Equal([]string{"oom", "party"}))
			Expect(response.GetHostIp()).To(Equal("host-ip"))
			Expect(response.GetContainerIp()).To(Equal("container-ip"))
			Expect(response.GetContainerPath()).To(Equal("/path/to/container"))
			Expect(response.GetProcessIds()).To(Equal([]uint64{1, 2}))

			Expect(response.GetMemoryStat().GetCache()).To(Equal(uint64(1)))
			Expect(response.GetMemoryStat().GetRss()).To(Equal(uint64(2)))
			Expect(response.GetMemoryStat().GetMappedFile()).To(Equal(uint64(3)))
			Expect(response.GetMemoryStat().GetPgpgin()).To(Equal(uint64(4)))
			Expect(response.GetMemoryStat().GetPgpgout()).To(Equal(uint64(5)))
			Expect(response.GetMemoryStat().GetSwap()).To(Equal(uint64(6)))
			Expect(response.GetMemoryStat().GetPgfault()).To(Equal(uint64(7)))
			Expect(response.GetMemoryStat().GetPgmajfault()).To(Equal(uint64(8)))
			Expect(response.GetMemoryStat().GetInactiveAnon()).To(Equal(uint64(9)))
			Expect(response.GetMemoryStat().GetActiveAnon()).To(Equal(uint64(10)))
			Expect(response.GetMemoryStat().GetInactiveFile()).To(Equal(uint64(11)))
			Expect(response.GetMemoryStat().GetActiveFile()).To(Equal(uint64(12)))
			Expect(response.GetMemoryStat().GetUnevictable()).To(Equal(uint64(13)))
			Expect(response.GetMemoryStat().GetHierarchicalMemoryLimit()).To(Equal(uint64(14)))
			Expect(response.GetMemoryStat().GetHierarchicalMemswLimit()).To(Equal(uint64(15)))
			Expect(response.GetMemoryStat().GetTotalCache()).To(Equal(uint64(16)))
			Expect(response.GetMemoryStat().GetTotalRss()).To(Equal(uint64(17)))
			Expect(response.GetMemoryStat().GetTotalMappedFile()).To(Equal(uint64(18)))
			Expect(response.GetMemoryStat().GetTotalPgpgin()).To(Equal(uint64(19)))
			Expect(response.GetMemoryStat().GetTotalPgpgout()).To(Equal(uint64(20)))
			Expect(response.GetMemoryStat().GetTotalSwap()).To(Equal(uint64(21)))
			Expect(response.GetMemoryStat().GetTotalPgfault()).To(Equal(uint64(22)))
			Expect(response.GetMemoryStat().GetTotalPgmajfault()).To(Equal(uint64(23)))
			Expect(response.GetMemoryStat().GetTotalInactiveAnon()).To(Equal(uint64(24)))
			Expect(response.GetMemoryStat().GetTotalActiveAnon()).To(Equal(uint64(25)))
			Expect(response.GetMemoryStat().GetTotalInactiveFile()).To(Equal(uint64(26)))
			Expect(response.GetMemoryStat().GetTotalActiveFile()).To(Equal(uint64(27)))
			Expect(response.GetMemoryStat().GetTotalUnevictable()).To(Equal(uint64(28)))

			Expect(response.GetCpuStat().GetUsage()).To(Equal(uint64(1)))
			Expect(response.GetCpuStat().GetUser()).To(Equal(uint64(2)))
			Expect(response.GetCpuStat().GetSystem()).To(Equal(uint64(3)))

			Expect(response.GetDiskStat().GetBytesUsed()).To(Equal(uint64(1)))
			Expect(response.GetDiskStat().GetInodesUsed()).To(Equal(uint64(2)))

			Expect(response.GetBandwidthStat().GetInRate()).To(Equal(uint64(1)))
			Expect(response.GetBandwidthStat().GetInBurst()).To(Equal(uint64(2)))
			Expect(response.GetBandwidthStat().GetOutRate()).To(Equal(uint64(3)))
			Expect(response.GetBandwidthStat().GetOutBurst()).To(Equal(uint64(4)))

			Expect(response.GetMappedPorts()).To(HaveLen(2))
			Expect(response.GetMappedPorts()[0].GetHostPort()).To(Equal(uint32(1234)))
			Expect(response.GetMappedPorts()[0].GetContainerPort()).To(Equal(uint32(5678)))
			Expect(response.GetMappedPorts()[1].GetHostPort()).To(Equal(uint32(1235)))
			Expect(response.GetMappedPorts()[1].GetContainerPort()).To(Equal(uint32(5679)))
			close(done)
		}, 1.0)

		It("includes the container's properties", func() {
			fakeContainer.ReportedInfo = warden.ContainerInfo{
				Properties: warden.Properties{
					"foo": "bar",
					"a":   "b",
				},
			}

			writeMessages(&protocol.InfoRequest{
				Handle: proto.String(fakeContainer.Handle()),
			})

			var response protocol.InfoResponse
			readResponse(&response)

			Expect(response.GetProperties()).To(ContainElement(&protocol.Property{
				Key:   proto.String("foo"),
				Value: proto.String("bar"),
			}))

			Expect(response.GetProperties()).To(ContainElement(&protocol.Property{
				Key:   proto.String("a"),
				Value: proto.String("b"),
			}))

			Expect(response.GetProperties()).To(HaveLen(2))
		})

		itResetsGraceTimeWhenHandling(&protocol.InfoRequest{
			Handle: proto.String("some-handle"),
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				serverBackend.Destroy(fakeContainer.Handle())
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.InfoRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.InfoResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{
					Message: "unknown handle: some-handle",
				}))

				close(done)
			}, 1.0)
		})

		Context("when getting container info fails", func() {
			BeforeEach(func() {
				fakeContainer.InfoError = errors.New("oh no!")
			})

			It("sends a WardenError response", func(done Done) {
				writeMessages(&protocol.InfoRequest{
					Handle: proto.String(fakeContainer.Handle()),
				})

				var response protocol.InfoResponse
				err := transport.ReadMessage(responses, &response)
				Expect(err).To(Equal(&transport.WardenError{Message: "oh no!"}))

				close(done)
			}, 1.0)
		})
	})
})
