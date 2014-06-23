package client_test

import (
	"bytes"
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"strings"

	. "github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fake_connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var _ = Describe("Container", func() {
	var container warden.Container

	var fakeConnection *fake_connection.FakeConnection

	BeforeEach(func() {
		fakeConnection = fake_connection.New()
	})

	JustBeforeEach(func() {
		var err error

		client := New(fakeConnection)

		fakeConnection.WhenCreating = func(warden.ContainerSpec) (string, error) {
			return "some-handle", nil
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
					Kill: true,
				},
			))
		})

		Context("when stopping fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenStopping = func(handle string, kill bool) error {
					return disaster
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
			infoToReturn := warden.ContainerInfo{
				State: "chillin",
			}

			fakeConnection.WhenGettingInfo = func(handle string) (warden.ContainerInfo, error) {
				Ω(handle).Should(Equal("some-handle"))
				return infoToReturn, nil
			}

			info, err := container.Info()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(info).Should(Equal(infoToReturn))
		})

		Context("when getting info fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingInfo = func(handle string) (warden.ContainerInfo, error) {
					return warden.ContainerInfo{}, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.Info()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("StreamIn", func() {
		It("sends a stream in request", func() {
			fakeConnection.WhenStreamingIn = func(handle string, dst string, reader io.Reader) error {
				Ω(dst).Should(Equal("to"))

				content, err := ioutil.ReadAll(reader)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(content)).Should(Equal("stuff"))

				return nil
			}

			err := container.StreamIn("to", bytes.NewBufferString("stuff"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when streaming in fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenStreamingIn = func(handle string, dst string, src io.Reader) error {
					return disaster
				}
			})

			It("returns the error", func() {
				err := container.StreamIn("to", nil)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("StreamOut", func() {
		It("sends a stream out request", func() {
			fakeConnection.WhenStreamingOut = func(handle string, src string) (io.ReadCloser, error) {
				Ω(src).Should(Equal("from"))
				return ioutil.NopCloser(strings.NewReader("kewl")), nil
			}

			reader, err := container.StreamOut("from")
			bytes, err := ioutil.ReadAll(reader)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(string(bytes)).Should(Equal("kewl"))
		})

		Context("when streaming out fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenStreamingOut = func(handle string, src string) (io.ReadCloser, error) {
					return nil, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.StreamOut("from")
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
				fakeConnection.WhenLimitingBandwidth = func(handle string, limits warden.BandwidthLimits) (warden.BandwidthLimits, error) {
					return warden.BandwidthLimits{}, disaster
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
				fakeConnection.WhenLimitingCPU = func(handle string, limits warden.CPULimits) (warden.CPULimits, error) {
					return warden.CPULimits{}, disaster
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
				ByteHard: 1,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.LimitedDisk("some-handle")).Should(ContainElement(
				warden.DiskLimits{
					ByteHard: 1,
				},
			))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenLimitingDisk = func(handle string, limits warden.DiskLimits) (warden.DiskLimits, error) {
					return warden.DiskLimits{}, disaster
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
				fakeConnection.WhenLimitingMemory = func(handle string, limits warden.MemoryLimits) (warden.MemoryLimits, error) {
					return warden.MemoryLimits{}, disaster
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
			limitsToReturn := warden.BandwidthLimits{
				RateInBytesPerSecond:      1,
				BurstRateInBytesPerSecond: 2,
			}

			fakeConnection.WhenGettingCurrentBandwidthLimits = func(handle string) (warden.BandwidthLimits, error) {
				return limitsToReturn, nil
			}

			limits, err := container.CurrentBandwidthLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(limitsToReturn))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingCurrentBandwidthLimits = func(handle string) (warden.BandwidthLimits, error) {
					return warden.BandwidthLimits{}, disaster
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
			limitsToReturn := warden.CPULimits{
				LimitInShares: 1,
			}

			fakeConnection.WhenGettingCurrentCPULimits = func(handle string) (warden.CPULimits, error) {
				return limitsToReturn, nil
			}

			limits, err := container.CurrentCPULimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(limitsToReturn))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingCurrentCPULimits = func(handle string) (warden.CPULimits, error) {
					return warden.CPULimits{}, disaster
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
			limitsToReturn := warden.DiskLimits{
				BlockSoft: 3,
				BlockHard: 4,
				InodeSoft: 7,
				InodeHard: 8,
				ByteSoft:  11,
				ByteHard:  12,
			}

			fakeConnection.WhenGettingCurrentDiskLimits = func(handle string) (warden.DiskLimits, error) {
				return limitsToReturn, nil
			}

			limits, err := container.CurrentDiskLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(limitsToReturn))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingCurrentDiskLimits = func(handle string) (warden.DiskLimits, error) {
					return warden.DiskLimits{}, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentDiskLimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("CurrentMemoryLimits", func() {
		It("gets the current limits", func() {
			limitsToReturn := warden.MemoryLimits{
				LimitInBytes: 1,
			}

			fakeConnection.WhenGettingCurrentMemoryLimits = func(handle string) (warden.MemoryLimits, error) {
				return limitsToReturn, nil
			}

			limits, err := container.CurrentMemoryLimits()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(limits).Should(Equal(limitsToReturn))
		})

		Context("when the request fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingCurrentMemoryLimits = func(handle string) (warden.MemoryLimits, error) {
					return warden.MemoryLimits{}, disaster
				}
			})

			It("returns the error", func() {
				_, err := container.CurrentMemoryLimits()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Run", func() {
		It("sends a run request and returns the process id and a stream", func() {
			fakeConnection.WhenRunning = func(handle string, spec warden.ProcessSpec) (uint32, <-chan warden.ProcessStream, error) {
				stream := make(chan warden.ProcessStream, 3)

				stream <- warden.ProcessStream{
					Source: warden.ProcessStreamSourceStdout,
					Data:   []byte("stdout data"),
				}

				stream <- warden.ProcessStream{
					Source: warden.ProcessStreamSourceStderr,
					Data:   []byte("stderr data"),
				}

				exitStatus := uint32(123)
				stream <- warden.ProcessStream{
					ExitStatus: &exitStatus,
				}

				close(stream)

				return 42, stream, nil
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
	})

	Describe("Attach", func() {
		It("sends an attach request and returns a stream", func() {
			fakeConnection.WhenAttaching = func(handle string, processID uint32) (<-chan warden.ProcessStream, error) {
				stream := make(chan warden.ProcessStream, 3)

				stream <- warden.ProcessStream{
					Source: warden.ProcessStreamSourceStdout,
					Data:   []byte("stdout data"),
				}

				stream <- warden.ProcessStream{
					Source: warden.ProcessStreamSourceStderr,
					Data:   []byte("stderr data"),
				}

				exitStatus := uint32(123)
				stream <- warden.ProcessStream{
					ExitStatus: &exitStatus,
				}

				close(stream)

				return stream, nil
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
	})

	Describe("NetIn", func() {
		It("sends a net in request", func() {
			fakeConnection.WhenNetInning = func(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
				return 111, 222, nil
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
				fakeConnection.WhenNetInning = func(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
					return 0, 0, disaster
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
				fakeConnection.WhenNetOuting = func(handle string, network string, port uint32) error {
					return disaster
				}
			})

			It("returns the error", func() {
				err := container.NetOut("some-network", 1234)
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
