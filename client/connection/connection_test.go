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
			handle, err := connection.Create(warden.ContainerSpec{
				Handle:     "some-handle",
				GraceTime:  10 * time.Second,
				RootFSPath: "some-rootfs-path",
				Network:    "some-network",
				BindMounts: []warden.BindMount{
					{
						SrcPath: "/src-a",
						DstPath: "/dst-a",
						Mode:    warden.BindMountModeRO,
						Origin:  warden.BindMountOriginHost,
					},
					{
						SrcPath: "/src-b",
						DstPath: "/dst-b",
						Mode:    warden.BindMountModeRW,
						Origin:  warden.BindMountOriginContainer,
					},
				},
				Properties: map[string]string{
					"foo": "bar",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(handle).Should(Equal("foohandle"))

			ro := protocol.CreateRequest_BindMount_RO
			rw := protocol.CreateRequest_BindMount_RW

			hostOrigin := protocol.CreateRequest_BindMount_Host
			containerOrigin := protocol.CreateRequest_BindMount_Container

			assertWriteBufferContains(&protocol.CreateRequest{
				Handle:    proto.String("some-handle"),
				GraceTime: proto.Uint32(10),
				Rootfs:    proto.String("some-rootfs-path"),
				Network:   proto.String("some-network"),
				BindMounts: []*protocol.CreateRequest_BindMount{
					{
						SrcPath: proto.String("/src-a"),
						DstPath: proto.String("/dst-a"),
						Mode:    &ro,
						Origin:  &hostOrigin,
					},
					{
						SrcPath: proto.String("/src-b"),
						DstPath: proto.String("/dst-b"),
						Mode:    &rw,
						Origin:  &containerOrigin,
					},
				},
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
			err := connection.Stop("foo", true, true)
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
			err := connection.Destroy("foo")
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
				newLimits, err := connection.LimitMemory("foo", warden.MemoryLimits{
					LimitInBytes: 42,
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.LimitInBytes).Should(BeNumerically("==", 40))

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
			newLimits, err := connection.LimitCPU("foo", warden.CPULimits{
				LimitInShares: 42,
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(newLimits.LimitInShares).Should(BeNumerically("==", 40))

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
			newLimits, err := connection.LimitBandwidth("foo", warden.BandwidthLimits{
				RateInBytesPerSecond:      42,
				BurstRateInBytesPerSecond: 43,
			})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(newLimits.RateInBytesPerSecond).Should(BeNumerically("==", 1))
			Ω(newLimits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 2))

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
					&protocol.LimitDiskResponse{
						BlockLimit: proto.Uint64(1),
						Block:      proto.Uint64(2),
						BlockSoft:  proto.Uint64(3),
						BlockHard:  proto.Uint64(4),
						InodeLimit: proto.Uint64(5),
						Inode:      proto.Uint64(6),
						InodeSoft:  proto.Uint64(7),
						InodeHard:  proto.Uint64(8),
						ByteLimit:  proto.Uint64(9),
						Byte:       proto.Uint64(10),
						ByteSoft:   proto.Uint64(11),
						ByteHard:   proto.Uint64(12),
					},
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

				newLimits, err := connection.LimitDisk("foo", limits)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(newLimits).Should(Equal(warden.DiskLimits{
					BlockLimit: 1,
					Block:      2,
					BlockSoft:  3,
					BlockHard:  4,
					InodeLimit: 5,
					Inode:      6,
					InodeSoft:  7,
					InodeHard:  8,
					ByteLimit:  9,
					Byte:       10,
					ByteSoft:   11,
					ByteHard:   12,
				}))

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
			err := connection.NetOut("foo-handle", "foo-network", 42)
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
			handles, err := connection.List(map[string]string{"foo": "bar"})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(handles).Should(Equal([]string{"container1", "container2", "container3"}))

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
					State:         proto.String("chilling out"),
					Events:        []string{"maxing", "relaxing all cool"},
					HostIp:        proto.String("host-ip"),
					ContainerIp:   proto.String("container-ip"),
					ContainerPath: proto.String("container-path"),
					ProcessIds:    []uint64{1, 2},

					Properties: []*protocol.Property{
						{
							Key:   proto.String("proto-key"),
							Value: proto.String("proto-value"),
						},
					},

					MemoryStat: &protocol.InfoResponse_MemoryStat{
						Cache:                   proto.Uint64(1),
						Rss:                     proto.Uint64(2),
						MappedFile:              proto.Uint64(3),
						Pgpgin:                  proto.Uint64(4),
						Pgpgout:                 proto.Uint64(5),
						Swap:                    proto.Uint64(6),
						Pgfault:                 proto.Uint64(7),
						Pgmajfault:              proto.Uint64(8),
						InactiveAnon:            proto.Uint64(9),
						ActiveAnon:              proto.Uint64(10),
						InactiveFile:            proto.Uint64(11),
						ActiveFile:              proto.Uint64(12),
						Unevictable:             proto.Uint64(13),
						HierarchicalMemoryLimit: proto.Uint64(14),
						HierarchicalMemswLimit:  proto.Uint64(15),
						TotalCache:              proto.Uint64(16),
						TotalRss:                proto.Uint64(17),
						TotalMappedFile:         proto.Uint64(18),
						TotalPgpgin:             proto.Uint64(19),
						TotalPgpgout:            proto.Uint64(20),
						TotalSwap:               proto.Uint64(21),
						TotalPgfault:            proto.Uint64(22),
						TotalPgmajfault:         proto.Uint64(23),
						TotalInactiveAnon:       proto.Uint64(24),
						TotalActiveAnon:         proto.Uint64(25),
						TotalInactiveFile:       proto.Uint64(26),
						TotalActiveFile:         proto.Uint64(27),
						TotalUnevictable:        proto.Uint64(28),
					},

					CpuStat: &protocol.InfoResponse_CpuStat{
						Usage:  proto.Uint64(1),
						User:   proto.Uint64(2),
						System: proto.Uint64(3),
					},

					DiskStat: &protocol.InfoResponse_DiskStat{
						BytesUsed:  proto.Uint64(1),
						InodesUsed: proto.Uint64(2),
					},

					BandwidthStat: &protocol.InfoResponse_BandwidthStat{
						InRate:   proto.Uint64(1),
						InBurst:  proto.Uint64(2),
						OutRate:  proto.Uint64(3),
						OutBurst: proto.Uint64(4),
					},
				},
			)
		})

		It("should return the container's info", func() {
			info, err := connection.Info("handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(info.State).Should(Equal("chilling out"))
			Ω(info.Events).Should(Equal([]string{"maxing", "relaxing all cool"}))
			Ω(info.HostIP).Should(Equal("host-ip"))
			Ω(info.ContainerIP).Should(Equal("container-ip"))
			Ω(info.ContainerPath).Should(Equal("container-path"))
			Ω(info.ProcessIDs).Should(Equal([]uint32{1, 2}))

			Ω(info.MemoryStat).Should(Equal(warden.ContainerMemoryStat{
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
			}))

			Ω(info.CPUStat).Should(Equal(warden.ContainerCPUStat{
				Usage:  1,
				User:   2,
				System: 3,
			}))

			Ω(info.DiskStat).Should(Equal(warden.ContainerDiskStat{
				BytesUsed:  1,
				InodesUsed: 2,
			}))

			Ω(info.BandwidthStat).Should(Equal(warden.ContainerBandwidthStat{
				InRate:   1,
				InBurst:  2,
				OutRate:  3,
				OutBurst: 4,
			}))

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
			err := connection.CopyIn("foo-handle", "/foo", "/bar")
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
			err := connection.CopyOut("foo-handle", "/foo", "/bar", "bartholofoo")
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

			err := connection.Destroy("foo-handle")
			Ω(err).ShouldNot(HaveOccurred())

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
				pid, stream, err := connection.Run("foo-handle", warden.ProcessSpec{
					Script: "lol",
					Limits: resourceLimits,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(pid).Should(BeNumerically("==", 42))

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
				Ω(response1.Source).Should(Equal(warden.ProcessStreamSourceStdout))
				Ω(string(response1.Data)).Should(Equal("1"))

				response2 := <-stream
				Ω(response2.Source).Should(Equal(warden.ProcessStreamSourceStderr))
				Ω(string(response2.Data)).Should(Equal("2"))

				response3 := <-stream
				Ω(response3.ExitStatus).ShouldNot(BeNil())
				Ω(*response3.ExitStatus).Should(BeNumerically("==", 3))

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
				pid, _, err := connection.Run("foo-handle", warden.ProcessSpec{
					Script: "echo hi",
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(pid).Should(BeNumerically("==", 42))

				assertWriteBufferContains(&protocol.RunRequest{
					Handle:  proto.String("foo-handle"),
					Script:  proto.String("echo hi"),
					Rlimits: &protocol.ResourceLimits{},
				})

				writeBuffer.Reset()

				time.Sleep(1 * time.Second)

				pid, _, err = connection.Run("foo-handle", warden.ProcessSpec{
					Script: "echo bye",
				})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(pid).Should(BeNumerically("==", 43))

				assertWriteBufferContains(&protocol.RunRequest{
					Handle:  proto.String("foo-handle"),
					Script:  proto.String("echo bye"),
					Rlimits: &protocol.ResourceLimits{},
				})
			})
		})
	})

	Describe("Attaching", func() {
		BeforeEach(func() {
			stdout := protocol.ProcessPayload_stdout
			stderr := protocol.ProcessPayload_stderr

			wardenMessages = append(wardenMessages,
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stdout, Data: proto.String("1")},
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), Source: &stderr, Data: proto.String("2")},
				&protocol.ProcessPayload{ProcessId: proto.Uint32(42), ExitStatus: proto.Uint32(3)},
			)
		})

		It("should stream", func(done Done) {
			stream, err := connection.Attach("foo-handle", 42)
			Ω(err).ShouldNot(HaveOccurred())

			assertWriteBufferContains(&protocol.AttachRequest{
				Handle:    proto.String("foo-handle"),
				ProcessId: proto.Uint32(42),
			})

			response1 := <-stream
			Ω(response1.Source).Should(Equal(warden.ProcessStreamSourceStdout))
			Ω(string(response1.Data)).Should(Equal("1"))

			response2 := <-stream
			Ω(response2.Source).Should(Equal(warden.ProcessStreamSourceStderr))
			Ω(string(response2.Data)).Should(Equal("2"))

			response3 := <-stream
			Ω(response3.ExitStatus).ShouldNot(BeNil())
			Ω(*response3.ExitStatus).Should(BeNumerically("==", 3))

			Eventually(stream).Should(BeClosed())

			close(done)
		})
	})
})
