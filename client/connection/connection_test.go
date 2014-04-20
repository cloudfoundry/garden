package connection_test

import (
	"bufio"
	"bytes"
	"net"
	"time"

	"code.google.com/p/gogoprotobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/message_reader"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
	. "github.com/cloudfoundry-incubator/gordon/test_helpers" // TODO
)

var _ = Describe("Info", func() {
	var listener net.Listener

	BeforeEach(func() {
		var err error

		listener, err = net.Listen("tcp", ":0")
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("should connect to the listener", func() {
		info := &Info{
			Network: listener.Addr().Network(),
			Addr:    listener.Addr().String(),
		}

		conn, err := info.ProvideConnection()
		Ω(conn).ShouldNot(BeNil())
		Ω(err).ShouldNot(HaveOccurred())
	})
})

var _ = Describe("Connection", func() {
	var (
		connection     Connection
		writeBuffer    *bytes.Buffer
		wardenMessages []proto.Message
		resourceLimits warden.ResourceLimits
	)

	assertWriteBufferContains := func(messages ...proto.Message) {
		reader := bufio.NewReader(bytes.NewBuffer(writeBuffer.Bytes()))

		for _, msg := range messages {
			req, err := message_reader.ReadRequest(reader)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(req).Should(Equal(msg))
		}
	}

	JustBeforeEach(func() {
		writeBuffer = bytes.NewBuffer([]byte{})

		fakeConn := &FakeConn{
			ReadBuffer:  protocol.Messages(wardenMessages...),
			WriteBuffer: writeBuffer,
		}

		connection = New(fakeConn)
	})

	BeforeEach(func() {
		wardenMessages = []proto.Message{}

		rlimits := &warden.ResourceLimits{
			As:         proto.Uint64(1),
			Core:       proto.Uint64(2),
			Cpu:        proto.Uint64(4),
			Data:       proto.Uint64(5),
			Fsize:      proto.Uint64(6),
			Locks:      proto.Uint64(7),
			Memlock:    proto.Uint64(8),
			Msgqueue:   proto.Uint64(9),
			Nice:       proto.Uint64(10),
			Nofile:     proto.Uint64(11),
			Nproc:      proto.Uint64(12),
			Rss:        proto.Uint64(13),
			Rtprio:     proto.Uint64(14),
			Sigpending: proto.Uint64(15),
			Stack:      proto.Uint64(16),
		}

		resourceLimits = warden.ResourceLimits{
			As:         rlimits.As,
			Core:       rlimits.Core,
			Cpu:        rlimits.Cpu,
			Data:       rlimits.Data,
			Fsize:      rlimits.Fsize,
			Locks:      rlimits.Locks,
			Memlock:    rlimits.Memlock,
			Msgqueue:   rlimits.Msgqueue,
			Nice:       rlimits.Nice,
			Nofile:     rlimits.Nofile,
			Nproc:      rlimits.Nproc,
			Rss:        rlimits.Rss,
			Rtprio:     rlimits.Rtprio,
			Sigpending: rlimits.Sigpending,
			Stack:      rlimits.Stack,
		}
	})

	Describe("Creating", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.CreateResponse{
					Handle: proto.String("foohandle"),
				},
			)
		})

		It("should create a container", func() {
			resp, err := connection.Create(warden.ContainerSpec{
				Properties: map[string]string{
					"foo": "bar",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(resp.GetHandle()).Should(Equal("foohandle"))

			assertWriteBufferContains(&protocol.CreateRequest{
				Properties: []*protocol.Property{
					{
						Key:   proto.String("foo"),
						Value: proto.String("bar"),
					},
				},
			})
		})
	})

	Describe("Stopping", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.StopResponse{},
			)
		})

		It("should stop the container", func() {
			_, err := connection.Stop("foo", true, true)
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.StopRequest{
				Handle:     proto.String("foo"),
				Background: proto.Bool(true),
				Kill:       proto.Bool(true),
			})
		})
	})

	Describe("Destroying", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.DestroyResponse{},
			)
		})

		It("should stop the container", func() {
			_, err := connection.Destroy("foo")
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.DestroyRequest{
				Handle: proto.String("foo"),
			})
		})
	})

	Describe("Limiting Memory", func() {
		Describe("Setting the memory limit", func() {
			BeforeEach(func() {
				wardenMessages = append(wardenMessages,
					&protocol.LimitMemoryResponse{LimitInBytes: proto.Uint64(40)},
				)
			})

			It("should limit memory", func() {
				res, err := connection.LimitMemory("foo", warden.MemoryLimits{
					LimitInBytes: 42,
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(res.GetLimitInBytes()).Should(BeNumerically("==", 40))

				assertWriteBufferContains(&protocol.LimitMemoryRequest{
					Handle:       proto.String("foo"),
					LimitInBytes: proto.Uint64(42),
				})
			})
		})
	})

	Describe("Limiting CPU", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.LimitCpuResponse{LimitInShares: proto.Uint64(40)},
			)
		})

		It("should limit CPU", func() {
			res, err := connection.LimitCPU("foo", warden.CPULimits{
				LimitInShares: 42,
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res.GetLimitInShares()).Should(BeNumerically("==", 40))

			assertWriteBufferContains(&protocol.LimitCpuRequest{
				Handle:        proto.String("foo"),
				LimitInShares: proto.Uint64(42),
			})
		})
	})

	Describe("Limiting Bandwidth", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.LimitBandwidthResponse{
					Rate:  proto.Uint64(1),
					Burst: proto.Uint64(2),
				},
			)
		})

		It("should limit Bandwidth", func() {
			res, err := connection.LimitBandwidth("foo", warden.BandwidthLimits{
				RateInBytesPerSecond:      42,
				BurstRateInBytesPerSecond: 43,
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(res.GetRate()).Should(BeNumerically("==", 1))
			Ω(res.GetBurst()).Should(BeNumerically("==", 2))

			assertWriteBufferContains(&protocol.LimitBandwidthRequest{
				Handle: proto.String("foo"),
				Rate:   proto.Uint64(42),
				Burst:  proto.Uint64(43),
			})
		})
	})

	Describe("Limiting Disk", func() {
		Describe("Setting the disk limit", func() {
			BeforeEach(func() {
				wardenMessages = append(wardenMessages,
					&protocol.LimitDiskResponse{ByteLimit: proto.Uint64(40)},
				)
			})

			It("should limit disk", func() {
				limits := warden.DiskLimits{
					BlockLimit: 42,
					Block:      42,
					BlockSoft:  42,
					BlockHard:  42,

					InodeLimit: 42,
					Inode:      42,
					InodeSoft:  42,
					InodeHard:  42,

					ByteLimit: 42,
					Byte:      42,
					ByteSoft:  42,
					ByteHard:  42,
				}

				res, err := connection.LimitDisk("foo", limits)

				Ω(err).ShouldNot(HaveOccurred())
				Ω(res.GetByteLimit()).Should(BeNumerically("==", 40))

				assertWriteBufferContains(&protocol.LimitDiskRequest{
					Handle: proto.String("foo"),

					BlockLimit: proto.Uint64(limits.BlockLimit),
					Block:      proto.Uint64(limits.Block),
					BlockSoft:  proto.Uint64(limits.BlockSoft),
					BlockHard:  proto.Uint64(limits.BlockHard),

					InodeLimit: proto.Uint64(limits.InodeLimit),
					Inode:      proto.Uint64(limits.Inode),
					InodeSoft:  proto.Uint64(limits.InodeSoft),
					InodeHard:  proto.Uint64(limits.InodeHard),

					ByteLimit: proto.Uint64(limits.ByteLimit),
					Byte:      proto.Uint64(limits.Byte),
					ByteSoft:  proto.Uint64(limits.ByteSoft),
					ByteHard:  proto.Uint64(limits.ByteHard),
				})
			})
		})
	})

	Describe("NetOut", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.NetOutResponse{},
			)
		})

		It("should return the allocated ports", func() {
			_, err := connection.NetOut("foo-handle", "foo-network", 42)
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.NetOutRequest{
				Handle:  proto.String("foo-handle"),
				Network: proto.String("foo-network"),
				Port:    proto.Uint32(42),
			})
		})
	})

	Describe("Listing containers", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.ListResponse{
					Handles: []string{"container1", "container2", "container3"},
				},
			)
		})

		It("should return the list of containers", func() {
			resp, err := connection.List(map[string]string{"foo": "bar"})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(resp.GetHandles()).Should(Equal([]string{"container1", "container2", "container3"}))

			assertWriteBufferContains(&protocol.ListRequest{
				Properties: []*protocol.Property{
					{
						Key:   proto.String("foo"),
						Value: proto.String("bar"),
					},
				},
			})
		})
	})

	Describe("Getting container info", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.InfoResponse{
					State: proto.String("active"),
				},
			)
		})

		It("should return the container's info", func() {
			resp, err := connection.Info("handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(resp.GetState()).Should(Equal("active"))

			assertWriteBufferContains(&protocol.InfoRequest{
				Handle: proto.String("handle"),
			})
		})
	})

	Describe("Copying in", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.CopyInResponse{},
			)
		})

		It("should tell warden to copy in", func() {
			_, err := connection.CopyIn("foo-handle", "/foo", "/bar")
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.CopyInRequest{
				Handle:  proto.String("foo-handle"),
				SrcPath: proto.String("/foo"),
				DstPath: proto.String("/bar"),
			})
		})
	})

	Describe("Copying out", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.CopyOutResponse{},
			)
		})

		It("should tell warden to copy out", func() {
			_, err := connection.CopyOut("foo-handle", "/foo", "/bar", "bartholofoo")
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.CopyOutRequest{
				Handle:  proto.String("foo-handle"),
				SrcPath: proto.String("/foo"),
				DstPath: proto.String("/bar"),
				Owner:   proto.String("bartholofoo"),
			})
		})
	})

	Describe("When a connection error occurs", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.DestroyResponse{},
				//EOF
			)
		})

		It("should disconnect", func() {
			Ω(connection.Disconnected()).ShouldNot(BeClosed())

			resp, err := connection.Destroy("foo-handle")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(resp).ShouldNot(BeNil())

			Ω(connection.Disconnected()).Should(BeClosed())
		})
	})

	Describe("Disconnecting", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.ErrorResponse{Message: proto.String("boo")},
			)
		})

		It("should error", func() {
			processID, resp, err := connection.Run("foo-handle", warden.ProcessSpec{
				Script: "echo hi",
				Limits: resourceLimits,
			})
			Ω(processID).Should(BeZero())
			Ω(resp).Should(BeNil())
			Ω(err.Error()).Should(Equal("boo"))
		})
	})

	Describe("Round tripping", func() {
		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.EchoResponse{Message: proto.String("pong")},
			)
		})

		It("should do the round trip", func() {
			resp, err := connection.RoundTrip(
				&protocol.EchoRequest{Message: proto.String("ping")},
				&protocol.EchoResponse{},
			)

			Ω(err).ShouldNot(HaveOccurred())
			Ω(resp.(*protocol.EchoResponse).GetMessage()).Should(Equal("pong"))
		})
	})

	Describe("Running", func() {
		stdout := protocol.ProcessPayload_stdout
		stderr := protocol.ProcessPayload_stderr

		Context("when running one process", func() {
			BeforeEach(func() {
				wardenMessages = append(wardenMessages,
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42)},
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("1")},
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("2")},
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)},
				)
			})

			It("should start the process and stream output", func(done Done) {
				resp, stream, err := connection.Run("foo-handle", warden.ProcessSpec{
					Script: "lol",
					Limits: resourceLimits,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.GetProcessId()).Should(BeNumerically("==", 42))

				assertWriteBufferContains(&protocol.RunRequest{
					Handle: proto.String("foo-handle"),
					Script: proto.String("lol"),
					Rlimits: &protocol.ResourceLimits{
						As:         proto.Uint64(1),
						Core:       proto.Uint64(2),
						Cpu:        proto.Uint64(4),
						Data:       proto.Uint64(5),
						Fsize:      proto.Uint64(6),
						Locks:      proto.Uint64(7),
						Memlock:    proto.Uint64(8),
						Msgqueue:   proto.Uint64(9),
						Nice:       proto.Uint64(10),
						Nofile:     proto.Uint64(11),
						Nproc:      proto.Uint64(12),
						Rss:        proto.Uint64(13),
						Rtprio:     proto.Uint64(14),
						Sigpending: proto.Uint64(15),
						Stack:      proto.Uint64(16),
					},
				})

				response1 := <-stream
				Ω(response1.GetSource()).Should(Equal(stdout))
				Ω(response1.GetData()).Should(Equal("1"))

				response2 := <-stream
				Ω(response2.GetSource()).Should(Equal(stderr))
				Ω(response2.GetData()).Should(Equal("2"))

				response3, ok := <-stream
				Ω(response3.GetExitStatus()).Should(BeNumerically("==", 3))
				Ω(ok).Should(BeTrue())

				Eventually(stream).Should(BeClosed())

				close(done)
			})
		})

		Context("spawning multiple processes", func() {
			BeforeEach(func() {
				wardenMessages = append(wardenMessages,
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42)},
					//we do this, so that the first Run has some output to read......
					&protocol.ProcessPayload{ProcessId: proto.Uint32(42)},
					&protocol.ProcessPayload{ProcessId: proto.Uint32(43)},
				)
			})

			It("should be able to spawn multiple processes sequentially", func() {
				resp, _, err := connection.Run("foo-handle", warden.ProcessSpec{
					Script: "echo hi",
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.GetProcessId()).Should(BeNumerically("==", 42))

				assertWriteBufferContains(&protocol.RunRequest{
					Handle:  proto.String("foo-handle"),
					Script:  proto.String("echo hi"),
					Rlimits: &protocol.ResourceLimits{},
				})

				writeBuffer.Reset()

				time.Sleep(1 * time.Second)

				resp, _, err = connection.Run("foo-handle", warden.ProcessSpec{
					Script: "echo bye",
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.GetProcessId()).Should(BeNumerically("==", 43))

				assertWriteBufferContains(&protocol.RunRequest{
					Handle:  proto.String("foo-handle"),
					Script:  proto.String("echo bye"),
					Rlimits: &protocol.ResourceLimits{},
				})
			})
		})
	})

	Describe("Attaching", func() {
		stdout := protocol.ProcessPayload_stdout
		stderr := protocol.ProcessPayload_stderr

		BeforeEach(func() {
			wardenMessages = append(wardenMessages,
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("1")},
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("2")},
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)},
			)
		})

		It("should stream", func(done Done) {
			resp, err := connection.Attach("foo-handle", 42)
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.AttachRequest{
				Handle:    proto.String("foo-handle"),
				ProcessId: proto.Uint32(42),
			})

			response1 := <-resp
			Ω(response1.GetSource()).Should(Equal(protocol.ProcessPayload_stdout))
			Ω(response1.GetData()).Should(Equal("1"))

			response2 := <-resp
			Ω(response2.GetSource()).Should(Equal(protocol.ProcessPayload_stderr))
			Ω(response2.GetData()).Should(Equal("2"))

			response3 := <-resp
			Ω(response3.GetExitStatus()).Should(BeNumerically("==", 3))

			Eventually(resp).Should(BeClosed())

			close(done)
		})
	})
})
