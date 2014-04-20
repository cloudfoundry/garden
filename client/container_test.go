package client_test

import (
	"errors"

	"code.google.com/p/goprotobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/client/connection/fake_connection"
	protocol "github.com/cloudfoundry-incubator/garden/protocol"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var _ = Describe("Container", func() {
	var connectionProvider ConnectionProvider
	var container warden.Container

	var fakeConnection *fake_connection.FakeConnection

	BeforeEach(func() {
		fakeConnection = fake_connection.New()

		connectionProvider = &FakeConnectionProvider{
			Connection: fakeConnection,
		}
	})

	JustBeforeEach(func() {
		var err error

		client := New(connectionProvider)

		fakeConnection.WhenCreating = func(warden.ContainerSpec) (*protocol.CreateResponse, error) {
			return &protocol.CreateResponse{
				Handle: proto.String("some-handle"),
			}, nil
		}

		container, err = client.Create(warden.ContainerSpec{})
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("Handle", func() {
		It("returns the container's handle", func() {
			Ω(container.Handle()).Should(Equal("some-handle"))
		})
	})

	Describe("Stop", func() {
		It("sends a stop request", func() {
			err := container.Stop(true)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.Stopped("some-handle")).Should(ContainElement(
				fake_connection.StopSpec{
					Background: false,
					Kill:       true,
				},
			))
		})

		Context("when stopping fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenStopping = func(handle string, background, kill bool) (*protocol.StopResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.Stop(true)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Info", func() {
		It("sends an info request", func() {
			fakeConnection.WhenGettingInfo = func(handle string) (*protocol.InfoResponse, error) {
				Ω(handle).Should(Equal("some-handle"))

				return &protocol.InfoResponse{
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
				}, nil
			}

			info, err := container.Info()
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
		})

		Context("when getting info fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingInfo = func(handle string) (*protocol.InfoResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.Info()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CopyIn", func() {
		It("sends a copy in request", func() {
			err := container.CopyIn("from", "to")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.CopiedIn("some-handle")).Should(ContainElement(
				fake_connection.CopyInSpec{
					Source:      "from",
					Destination: "to",
				},
			))
		})

		Context("when copying in fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenCopyingIn = func(handle string, src, dst string) (*protocol.CopyInResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.CopyIn("from", "to")
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CopyOut", func() {
		It("sends a copy in request", func() {
			err := container.CopyOut("from", "to", "bob")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.CopiedOut("some-handle")).Should(ContainElement(
				fake_connection.CopyOutSpec{
					Source:      "from",
					Destination: "to",
					Owner:       "bob",
				},
			))
		})

		Context("when copying in fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenCopyingOut = func(handle string, src, dst, owner string) (*protocol.CopyOutResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.CopyOut("from", "to", "bob")
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("LimitBandwidth", func() {
		It("sends a limit bandwidth request", func() {
			err := container.LimitBandwidth(warden.BandwidthLimits{
				RateInBytesPerSecond: 1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.LimitedBandwidth("some-handle")).Should(ContainElement(
				warden.BandwidthLimits{
					RateInBytesPerSecond: 1,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingBandwidth = func(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.LimitBandwidth(warden.BandwidthLimits{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("LimitCPU", func() {
		It("sends a limit cpu request", func() {
			err := container.LimitCPU(warden.CPULimits{
				LimitInShares: 1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.LimitedCPU("some-handle")).Should(ContainElement(
				warden.CPULimits{
					LimitInShares: 1,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingCPU = func(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.LimitCPU(warden.CPULimits{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("LimitDisk", func() {
		It("sends a limit bandwidth request", func() {
			err := container.LimitDisk(warden.DiskLimits{
				ByteLimit: 1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.LimitedDisk("some-handle")).Should(ContainElement(
				warden.DiskLimits{
					ByteLimit: 1,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingDisk = func(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.LimitDisk(warden.DiskLimits{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("LimitMemory", func() {
		It("sends a limit bandwidth request", func() {
			err := container.LimitMemory(warden.MemoryLimits{
				LimitInBytes: 1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.LimitedMemory("some-handle")).Should(ContainElement(
				warden.MemoryLimits{
					LimitInBytes: 1,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingMemory = func(handle string, limits warden.MemoryLimits) (*protocol.LimitMemoryResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.LimitMemory(warden.MemoryLimits{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CurrentBandwidthLimits", func() {
		It("sends an empty limit request and returns its response", func() {
			fakeConnection.WhenLimitingBandwidth = func(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error) {
				return &protocol.LimitBandwidthResponse{
					Rate:  proto.Uint64(1),
					Burst: proto.Uint64(2),
				}, nil
			}

			limits, err := container.CurrentBandwidthLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(warden.BandwidthLimits{
				RateInBytesPerSecond:      1,
				BurstRateInBytesPerSecond: 2,
			}))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingBandwidth = func(handle string, limits warden.BandwidthLimits) (*protocol.LimitBandwidthResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentBandwidthLimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CurrentCPULimits", func() {
		It("sends an empty limit request and returns its response", func() {
			fakeConnection.WhenLimitingCPU = func(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error) {
				return &protocol.LimitCpuResponse{
					LimitInShares: proto.Uint64(1),
				}, nil
			}

			limits, err := container.CurrentCPULimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(warden.CPULimits{
				LimitInShares: 1,
			}))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingCPU = func(handle string, limits warden.CPULimits) (*protocol.LimitCpuResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentCPULimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CurrentDiskLimits", func() {
		It("sends an empty limit request and returns its response", func() {
			fakeConnection.WhenLimitingDisk = func(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error) {
				return &protocol.LimitDiskResponse{
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
				}, nil
			}

			limits, err := container.CurrentDiskLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(warden.DiskLimits{
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
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingDisk = func(handle string, limits warden.DiskLimits) (*protocol.LimitDiskResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentDiskLimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CurrentMemoryLimits", func() {
		It("sends an empty limit request and returns its response", func() {
			fakeConnection.WhenLimitingMemory = func(handle string, limits warden.MemoryLimits) (*protocol.LimitMemoryResponse, error) {
				return &protocol.LimitMemoryResponse{
					LimitInBytes: proto.Uint64(1),
				}, nil
			}

			limits, err := container.CurrentMemoryLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(warden.MemoryLimits{
				LimitInBytes: 1,
			}))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingMemory = func(handle string, limits warden.MemoryLimits) (*protocol.LimitMemoryResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentMemoryLimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Run", func() {
		var otherFakeConnection *fake_connection.FakeConnection

		BeforeEach(func() {
			otherFakeConnection = fake_connection.New()

			connectionProvider = &ManyConnectionProvider{
				Connections: []connection.Connection{
					fakeConnection,
					otherFakeConnection,
				},
			}
		})

		It("sends a run request and returns the process id and a stream", func() {
			fakeConnection.WhenRunning = func(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error) {
				payloads := make(chan *protocol.ProcessPayload, 3)

				stdout := protocol.ProcessPayload_stdout
				stderr := protocol.ProcessPayload_stderr

				payloads <- &protocol.ProcessPayload{
					Source: &stdout,
					Data:   proto.String("stdout data"),
				}

				payloads <- &protocol.ProcessPayload{
					Source: &stderr,
					Data:   proto.String("stderr data"),
				}

				payloads <- &protocol.ProcessPayload{
					ExitStatus: proto.Uint32(123),
				}

				return &protocol.ProcessPayload{
					ProcessId: proto.Uint32(42),
				}, payloads, nil
			}

			spec := warden.ProcessSpec{
				Script: "some-script",
			}

			pid, stream, err := container.Run(spec)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(pid).Should(Equal(uint32(42)))

			Ω(fakeConnection.SpawnedProcesses("some-handle")).Should(ContainElement(spec))

			Ω(<-stream).Should(Equal(warden.ProcessStream{
				Source: warden.ProcessStreamSourceStdout,
				Data:   []byte("stdout data"),
			}))

			Ω(<-stream).Should(Equal(warden.ProcessStream{
				Source: warden.ProcessStreamSourceStderr,
				Data:   []byte("stderr data"),
			}))

			exitStatus := uint32(123)
			Ω(<-stream).Should(Equal(warden.ProcessStream{
				ExitStatus: &exitStatus,
			}))

			Ω(stream).Should(BeClosed())
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenRunning = func(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error) {
					return nil, nil, disaster
				}
			})

			It("releases the connection", func() {
				_, _, err := container.Run(warden.ProcessSpec{})
				Ω(err).Should(Equal(disaster))

				err = container.Stop(false)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeConnection.Stopped("some-handle")).ShouldNot(BeEmpty())
			})
		})

		Context("while streaming", func() {
			It("does not permit reuse of the connection", func() {
				fakeConnection.WhenRunning = func(handle string, spec warden.ProcessSpec) (*protocol.ProcessPayload, <-chan *protocol.ProcessPayload, error) {
					payloads := make(chan *protocol.ProcessPayload)

					return &protocol.ProcessPayload{}, payloads, nil
				}

				_, _, err := container.Run(warden.ProcessSpec{})
				Ω(err).ShouldNot(HaveOccurred())

				err = container.Stop(false)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeConnection.Stopped("some-handle")).Should(BeEmpty())
				Ω(otherFakeConnection.Stopped("some-handle")).ShouldNot(BeEmpty())
			})
		})
	})

	Describe("Attach", func() {
		var otherFakeConnection *fake_connection.FakeConnection

		BeforeEach(func() {
			otherFakeConnection = fake_connection.New()

			connectionProvider = &ManyConnectionProvider{
				Connections: []connection.Connection{
					fakeConnection,
					otherFakeConnection,
				},
			}
		})

		It("sends an attach request and returns a stream", func() {
			fakeConnection.WhenAttaching = func(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error) {
				payloads := make(chan *protocol.ProcessPayload, 3)

				stdout := protocol.ProcessPayload_stdout
				stderr := protocol.ProcessPayload_stderr

				payloads <- &protocol.ProcessPayload{
					Source: &stdout,
					Data:   proto.String("stdout data"),
				}

				payloads <- &protocol.ProcessPayload{
					Source: &stderr,
					Data:   proto.String("stderr data"),
				}

				payloads <- &protocol.ProcessPayload{
					ExitStatus: proto.Uint32(123),
				}

				return payloads, nil
			}

			stream, err := container.Attach(42)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.AttachedProcesses("some-handle")).Should(ContainElement(uint32(42)))

			Ω(<-stream).Should(Equal(warden.ProcessStream{
				Source: warden.ProcessStreamSourceStdout,
				Data:   []byte("stdout data"),
			}))

			Ω(<-stream).Should(Equal(warden.ProcessStream{
				Source: warden.ProcessStreamSourceStderr,
				Data:   []byte("stderr data"),
			}))

			exitStatus := uint32(123)
			Ω(<-stream).Should(Equal(warden.ProcessStream{
				ExitStatus: &exitStatus,
			}))

			Ω(stream).Should(BeClosed())
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenAttaching = func(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error) {
					return nil, disaster
				}
			})

			It("releases the connection", func() {
				_, err := container.Attach(42)
				Ω(err).Should(Equal(disaster))

				err = container.Stop(false)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeConnection.Stopped("some-handle")).ShouldNot(BeEmpty())
			})
		})

		Context("while streaming", func() {
			It("does not permit reuse of the connection", func() {
				fakeConnection.WhenAttaching = func(handle string, processID uint32) (<-chan *protocol.ProcessPayload, error) {
					payloads := make(chan *protocol.ProcessPayload)

					return payloads, nil
				}

				_, err := container.Attach(42)
				Ω(err).ShouldNot(HaveOccurred())

				err = container.Stop(false)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeConnection.Stopped("some-handle")).Should(BeEmpty())
				Ω(otherFakeConnection.Stopped("some-handle")).ShouldNot(BeEmpty())
			})
		})
	})

	Describe("NetIn", func() {
		It("sends a net in request", func() {
			fakeConnection.WhenNetInning = func(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error) {
				return &protocol.NetInResponse{
					HostPort:      proto.Uint32(111),
					ContainerPort: proto.Uint32(222),
				}, nil
			}

			hostPort, containerPort, err := container.NetIn(123, 456)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(hostPort).Should(Equal(uint32(111)))
			Ω(containerPort).Should(Equal(uint32(222)))

			Ω(fakeConnection.NetInned("some-handle")).Should(ContainElement(
				fake_connection.NetInSpec{
					HostPort:      123,
					ContainerPort: 456,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenNetInning = func(handle string, hostPort, containerPort uint32) (*protocol.NetInResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, _, err := container.NetIn(123, 456)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("NetOut", func() {
		It("sends a net out request", func() {
			err := container.NetOut("some-network", 1234)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.NetOuted("some-handle")).Should(ContainElement(
				fake_connection.NetOutSpec{
					Network: "some-network",
					Port:    1234,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenNetOuting = func(handle string, network string, port uint32) (*protocol.NetOutResponse, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				err := container.NetOut("some-network", 1234)
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
