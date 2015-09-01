package server_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/cloudfoundry-incubator/garden/server"
)

var _ = Describe("When a client connects", func() {
	var socketPath string
	var tmpdir string

	var serverBackend *fakes.FakeBackend

	var serverContainerGraceTime time.Duration

	var logger *lagertest.TestLogger
	var sink *lagertest.TestSink

	var apiServer *server.GardenServer
	var apiClient garden.Client
	var isRunning bool

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		sink = lagertest.NewTestSink()
		logger.RegisterSink(sink)

		var err error
		tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
		Ω(err).ShouldNot(HaveOccurred())

		socketPath = path.Join(tmpdir, "api.sock")
		serverBackend = new(fakes.FakeBackend)
		serverContainerGraceTime = 42 * time.Second

		apiServer = server.New(
			"unix",
			socketPath,
			serverContainerGraceTime,
			serverBackend,
			logger,
		)

		err = apiServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		isRunning = true

		Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())

		apiClient = client.New(connection.New("unix", socketPath))
	})

	AfterEach(func() {
		if isRunning {
			apiServer.Stop()
		}
		if tmpdir != "" {
			os.RemoveAll(tmpdir)
		}
	})

	Context("and the client sends a PingRequest", func() {
		Context("and the backend ping succeeds", func() {
			It("does not error", func() {
				Ω(apiClient.Ping()).ShouldNot(HaveOccurred())
			})
		})

		Context("when the backend ping fails", func() {
			BeforeEach(func() {
				serverBackend.PingReturns(errors.New("oh no!"))
			})

			It("returns an error", func() {
				Ω(apiClient.Ping()).Should(HaveOccurred())
			})
		})

		Context("when the server is not up", func() {
			BeforeEach(func() {
				isRunning = false
				apiServer.Stop()
			})

			It("returns an error", func() {
				Ω(apiClient.Ping()).Should(HaveOccurred())
			})
		})
	})

	Context("and the client sends a CapacityRequest", func() {
		BeforeEach(func() {
			serverBackend.CapacityReturns(garden.Capacity{
				MemoryInBytes: 1111,
				DiskInBytes:   2222,
				MaxContainers: 42,
			}, nil)
		})

		It("returns the backend's reported capacity", func() {
			capacity, err := apiClient.Capacity()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(capacity.MemoryInBytes).Should(Equal(uint64(1111)))
			Ω(capacity.DiskInBytes).Should(Equal(uint64(2222)))
			Ω(capacity.MaxContainers).Should(Equal(uint64(42)))
		})

		Context("when getting the capacity fails", func() {
			BeforeEach(func() {
				serverBackend.CapacityReturns(garden.Capacity{}, errors.New("oh no!"))
			})

			It("returns an error", func() {
				_, err := apiClient.Capacity()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Context("and the client sends a CreateRequest", func() {
		var fakeContainer *fakes.FakeContainer

		BeforeEach(func() {
			fakeContainer = new(fakes.FakeContainer)
			fakeContainer.HandleReturns("some-handle")

			serverBackend.CreateReturns(fakeContainer, nil)
		})

		It("returns a container with the created handle", func() {
			container, err := apiClient.Create(garden.ContainerSpec{
				Handle: "some-handle",
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		It("should not log any container spec properties", func() {
			_, err := apiClient.Create(garden.ContainerSpec{
				Handle:     "some-handle",
				Properties: garden.Properties{"CONTAINER_PROPERTY": "CONTAINER_SECRET"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := sink.Buffer()
			Expect(buffer).ToNot(gbytes.Say("CONTAINER_PROPERTY"))
			Expect(buffer).ToNot(gbytes.Say("CONTAINER_SECRET"))
		})

		It("should not log any environment variables", func() {
			_, err := apiClient.Create(garden.ContainerSpec{
				Handle: "some-handle",
				Env:    []string{"PASSWORD=MY_SECRET"},
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := sink.Buffer()
			Expect(buffer).ToNot(gbytes.Say("PASSWORD"))
			Expect(buffer).ToNot(gbytes.Say("MY_SECRET"))
		})

		It("creates the container with the spec from the request", func() {
			_, err := apiClient.Create(garden.ContainerSpec{
				Handle:     "some-handle",
				GraceTime:  42 * time.Second,
				Network:    "some-network",
				RootFSPath: "/path/to/rootfs",
				BindMounts: []garden.BindMount{
					{
						SrcPath: "/bind/mount/src",
						DstPath: "/bind/mount/dst",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginContainer,
					},
				},
				Properties: garden.Properties{
					"prop-a": "val-a",
					"prop-b": "val-b",
				},
				Env: []string{"env1=env1Value", "env2=env2Value"},
				Limits: garden.Limits{
					Bandwidth: garden.BandwidthLimits{
						RateInBytesPerSecond:      42,
						BurstRateInBytesPerSecond: 68,
					},
					Disk: garden.DiskLimits{
						InodeSoft: 1,
						InodeHard: 2,
						ByteSoft:  3,
						ByteHard:  4,
						Scope:     garden.DiskLimitScopeExclusive,
					},
					Memory: garden.MemoryLimits{
						LimitInBytes: 1024,
					},
					CPU: garden.CPULimits{
						LimitInShares: 5,
					},
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(serverBackend.CreateArgsForCall(0)).Should(Equal(garden.ContainerSpec{
				Handle:     "some-handle",
				GraceTime:  time.Duration(42 * time.Second),
				Network:    "some-network",
				RootFSPath: "/path/to/rootfs",
				BindMounts: []garden.BindMount{
					{
						SrcPath: "/bind/mount/src",
						DstPath: "/bind/mount/dst",
						Mode:    garden.BindMountModeRW,
						Origin:  garden.BindMountOriginContainer,
					},
				},
				Properties: map[string]string{
					"prop-a": "val-a",
					"prop-b": "val-b",
				},
				Env: []string{"env1=env1Value", "env2=env2Value"},
				Limits: garden.Limits{
					Bandwidth: garden.BandwidthLimits{
						RateInBytesPerSecond:      42,
						BurstRateInBytesPerSecond: 68,
					},
					Disk: garden.DiskLimits{
						InodeSoft: 1,
						InodeHard: 2,
						ByteSoft:  3,
						ByteHard:  4,
						Scope:     garden.DiskLimitScopeExclusive,
					},
					Memory: garden.MemoryLimits{
						LimitInBytes: 1024,
					},
					CPU: garden.CPULimits{
						LimitInShares: 5,
					},
				},
			}))
		})

		Context("when a grace time is given", func() {
			It("destroys the container after it has been idle for the grace time", func() {
				graceTime := time.Second

				fakeContainer = new(fakes.FakeContainer)
				fakeContainer.HandleReturns("doomed-handle")

				serverBackend.GraceTimeReturns(graceTime)
				serverBackend.CreateReturns(fakeContainer, nil)
				serverBackend.LookupReturns(fakeContainer, nil)

				before := time.Now()

				_, err := apiClient.Create(garden.ContainerSpec{})
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(serverBackend.DestroyCallCount, 2*time.Second).Should(Equal(1))
				Ω(serverBackend.DestroyArgsForCall(0)).Should(Equal("doomed-handle"))

				Ω(time.Since(before)).Should(BeNumerically("~", graceTime, 100*time.Millisecond))
			})
		})

		Context("when a grace time is not given", func() {
			It("defaults it to the server's grace time", func() {
				_, err := apiClient.Create(garden.ContainerSpec{
					Handle: "some-handle",
				})
				Ω(err).ShouldNot(HaveOccurred())

				spec := serverBackend.CreateArgsForCall(0)
				Ω(spec.GraceTime).Should(Equal(serverContainerGraceTime))
			})
		})

		Context("when creating the container fails", func() {
			BeforeEach(func() {
				serverBackend.CreateReturns(nil, errors.New("oh no!"))
			})

			It("returns an error", func() {
				_, err := apiClient.Create(garden.ContainerSpec{
					Handle: "some-handle",
				})
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when creating the container fails with a ServiceUnavailableError", func() {
			var err error

			BeforeEach(func() {
				serverBackend.CreateReturns(nil, garden.NewServiceUnavailableError("special error"))

				_, err = apiClient.Create(garden.ContainerSpec{
					Handle: "some-handle",
				})
			})

			It("client returns an error with a well formed error msg", func() {
				Ω(err).Should(MatchError("special error"))
			})

			It("client returns an error of type ServiceUnavailableError", func() {
				_, ok := err.(*garden.ServiceUnavailableError)
				Ω(ok).Should(BeTrue())
			})
		})
	})

	Context("and the client sends a destroy request", func() {
		It("destroys the container", func() {
			err := apiClient.Destroy("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(serverBackend.DestroyArgsForCall(0)).Should(Equal("some-handle"))
		})

		Context("concurrent with other destroy requests", func() {
			var destroying chan struct{}

			BeforeEach(func() {
				destroying = make(chan struct{})

				serverBackend.DestroyStub = func(string) error {
					close(destroying)
					time.Sleep(time.Second)
					return nil
				}
			})

			It("only destroys once", func() {
				go apiClient.Destroy("some-handle")

				<-destroying

				err := apiClient.Destroy("some-handle")
				Ω(err).Should(HaveOccurred())

				Ω(serverBackend.DestroyCallCount()).Should(Equal(1))
			})
		})

		Context("when the container cannot be found", func() {
			var theError = garden.ContainerNotFoundError{"some-handle"}

			BeforeEach(func() {
				serverBackend.DestroyReturns(theError)
			})

			It("returns an ContainerNotFoundError", func() {
				err := apiClient.Destroy("some-handle")
				Ω(err).Should(MatchError(garden.ContainerNotFoundError{"some-handle"}))
			})
		})

		Context("when destroying the container fails", func() {
			var theError = errors.New("o no")

			BeforeEach(func() {
				serverBackend.DestroyReturns(theError)
			})

			It("returns an error with the same message", func() {
				err := apiClient.Destroy("some-handle")
				Ω(err).Should(MatchError("o no"))
			})

			Context("and destroying is attempted again", func() {
				BeforeEach(func() {
					err := apiClient.Destroy("some-handle")
					Ω(err).Should(HaveOccurred())

					serverBackend.DestroyReturns(nil)
				})

				It("tries to destroy again", func() {
					err := apiClient.Destroy("some-handle")
					Ω(err).ShouldNot(HaveOccurred())

					Ω(serverBackend.DestroyArgsForCall(0)).Should(Equal("some-handle"))
				})
			})
		})
	})

	Context("and the client sends a ListRequest", func() {
		BeforeEach(func() {
			c1 := new(fakes.FakeContainer)
			c1.HandleReturns("some-handle")

			c2 := new(fakes.FakeContainer)
			c2.HandleReturns("another-handle")

			c3 := new(fakes.FakeContainer)
			c3.HandleReturns("super-handle")

			serverBackend.ContainersReturns([]garden.Container{c1, c2, c3}, nil)
		})

		It("returns the containers from the backend", func() {
			containers, err := apiClient.Containers(nil)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(containers).Should(HaveLen(3))

			handles := make([]string, 3)
			for i, c := range containers {
				handles[i] = c.Handle()
			}

			Ω(handles).Should(ContainElement("some-handle"))
			Ω(handles).Should(ContainElement("another-handle"))
			Ω(handles).Should(ContainElement("super-handle"))
		})

		Context("when getting the containers fails", func() {
			BeforeEach(func() {
				serverBackend.ContainersReturns(nil, errors.New("oh no!"))
			})

			It("returns an error", func() {
				_, err := apiClient.Containers(nil)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("and the client sends a ListRequest with a property filter", func() {
			It("forwards the filter to the backend", func() {
				_, err := apiClient.Containers(garden.Properties{
					"foo": "bar",
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(serverBackend.ContainersArgsForCall(serverBackend.ContainersCallCount() - 1)).Should(Equal(
					garden.Properties{
						"foo": "bar",
					},
				))
			})
		})
	})

	Context("when a container has been created", func() {
		var container garden.Container

		var fakeContainer *fakes.FakeContainer

		BeforeEach(func() {
			fakeContainer = new(fakes.FakeContainer)
			fakeContainer.HandleReturns("some-handle")

			serverBackend.CreateReturns(fakeContainer, nil)
			serverBackend.LookupReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			var err error

			container, err = apiClient.Create(garden.ContainerSpec{})
			Ω(err).ShouldNot(HaveOccurred())
		})

		itResetsGraceTimeWhenHandling := func(call func()) {
			Context("when created with a grace time", func() {
				graceTime := 200 * time.Millisecond

				BeforeEach(func() {
					serverBackend.GraceTimeReturns(graceTime)
				})

				It("resets the container's grace time", func() {
					for i := 0; i < 11; i++ {
						time.Sleep(graceTime / 10)
						call()
					}

					before := time.Now()

					Eventually(serverBackend.DestroyCallCount, 2*graceTime).Should(Equal(1))
					Ω(serverBackend.DestroyArgsForCall(0)).Should(Equal(container.Handle()))

					Ω(time.Since(before)).Should(BeNumerically("~", graceTime, 30*time.Millisecond))
				})
			})
		}

		itFailsWhenTheContainerIsNotFound := func(example func() error) {
			Context("when the container is not found", func() {
				It("fails", func() {
					serverBackend.LookupReturns(nil, errors.New("not found"))
					Ω(example()).Should(MatchError("not found"))
				})
			})
		}

		Describe("stopping", func() {
			It("stops the container and sends a StopResponse", func() {
				err := container.Stop(true)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.StopArgsForCall(0)).Should(Equal(true))
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.Stop(true)
			})

			Context("when stopping the container fails", func() {
				BeforeEach(func() {
					fakeContainer.StopReturns(errors.New("oh no!"))
				})

				It("returns an error", func() {
					err := container.Stop(true)
					Ω(err).Should(HaveOccurred())
				})
			})

			itResetsGraceTimeWhenHandling(
				func() {
					err := container.Stop(false)
					Ω(err).ShouldNot(HaveOccurred())
				},
			)
		})

		Describe("metrics", func() {

			containerMetrics := garden.Metrics{
				MemoryStat: garden.ContainerMemoryStat{
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
					TotalUsageTowardLimit:   7, // TotalRss+(TotalCache-TotalInactiveFile)
				},
				CPUStat: garden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},
				DiskStat: garden.ContainerDiskStat{
					TotalBytesUsed:  1,
					TotalInodesUsed: 2,
				},
			}

			Context("when getting the metrics succeeds", func() {
				BeforeEach(func() {
					fakeContainer.MetricsReturns(
						containerMetrics,
						nil,
					)
				})

				It("returns the metrics from the container", func() {
					value, err := container.Metrics()
					Ω(err).ShouldNot(HaveOccurred())

					Ω(value).Should(Equal(containerMetrics))
				})

				itResetsGraceTimeWhenHandling(func() {
					_, err := container.Metrics()
					Ω(err).ShouldNot(HaveOccurred())
				})

				itFailsWhenTheContainerIsNotFound(func() error {
					_, err := container.Metrics()
					return err
				})
			})

			Context("when getting the metrics fails", func() {
				BeforeEach(func() {
					fakeContainer.MetricsReturns(garden.Metrics{}, errors.New("o no"))
				})

				It("returns an error", func() {
					metrics, err := container.Metrics()
					Ω(err).Should(HaveOccurred())
					Ω(metrics).Should(Equal(garden.Metrics{}))
				})
			})
		})

		Describe("properties", func() {
			Describe("getting all", func() {
				Context("when getting the properties succeeds", func() {
					BeforeEach(func() {
						fakeContainer.PropertiesReturns(garden.Properties{"foo": "bar"}, nil)
					})

					It("returns the properties from the container", func() {
						value, err := container.Properties()
						Ω(err).ShouldNot(HaveOccurred())

						Ω(value).Should(Equal(garden.Properties{"foo": "bar"}))
					})

					itResetsGraceTimeWhenHandling(func() {
						_, err := container.Properties()
						Ω(err).ShouldNot(HaveOccurred())
					})

					itFailsWhenTheContainerIsNotFound(func() error {
						_, err := container.Properties()
						return err
					})
				})

				Context("when getting the properties fails", func() {
					BeforeEach(func() {
						fakeContainer.PropertiesReturns(nil, errors.New("o no"))
					})

					It("returns an error", func() {
						properties, err := container.Properties()
						Ω(err).Should(HaveOccurred())
						Ω(properties).Should(BeEmpty())
					})
				})
			})

			Describe("getting", func() {
				Context("when getting the property succeeds", func() {
					BeforeEach(func() {
						fakeContainer.PropertyReturns("some-property-value", nil)
					})

					It("returns the property from the container", func() {
						value, err := container.Property("some-property")
						Ω(err).ShouldNot(HaveOccurred())

						Ω(value).Should(Equal("some-property-value"))

						Ω(fakeContainer.PropertyCallCount()).Should(Equal(1))

						name := fakeContainer.PropertyArgsForCall(0)
						Ω(name).Should(Equal("some-property"))
					})

					itResetsGraceTimeWhenHandling(func() {
						_, err := container.Property("some-property")
						Ω(err).ShouldNot(HaveOccurred())
					})

					itFailsWhenTheContainerIsNotFound(func() error {
						_, err := container.Property("some-property")
						return err
					})
				})

				Context("when getting the property fails", func() {
					BeforeEach(func() {
						fakeContainer.PropertyReturns("", errors.New("oh no!"))
					})

					It("returns an error", func() {
						value, err := container.Property("some-property")
						Ω(err).Should(HaveOccurred())
						Ω(value).Should(BeZero())
					})
				})
			})

			Describe("setting", func() {
				Context("when setting the property succeeds", func() {
					BeforeEach(func() {
						fakeContainer.SetPropertyReturns(nil)
					})

					It("sets the property on the container", func() {
						err := container.SetProperty("some-property", "some-value")
						Ω(err).ShouldNot(HaveOccurred())

						Ω(fakeContainer.SetPropertyCallCount()).Should(Equal(1))

						name, value := fakeContainer.SetPropertyArgsForCall(0)
						Ω(name).Should(Equal("some-property"))
						Ω(value).Should(Equal("some-value"))
					})

					itResetsGraceTimeWhenHandling(func() {
						err := container.SetProperty("some-property", "some-value")
						Ω(err).ShouldNot(HaveOccurred())
					})

					itFailsWhenTheContainerIsNotFound(func() error {
						return container.SetProperty("some-property", "some-value")
					})
				})

				Context("when setting the property fails", func() {
					BeforeEach(func() {
						fakeContainer.SetPropertyReturns(errors.New("oh no!"))
					})

					It("returns an error", func() {
						err := container.SetProperty("some-property", "some-value")
						Ω(err).Should(HaveOccurred())
					})
				})
			})

			Describe("removing", func() {
				Context("when removing the property succeeds", func() {
					BeforeEach(func() {
						fakeContainer.RemovePropertyReturns(nil)
					})

					It("returns the property from the container", func() {
						err := container.RemoveProperty("some-property")
						Ω(err).ShouldNot(HaveOccurred())

						Ω(fakeContainer.RemovePropertyCallCount()).Should(Equal(1))

						name := fakeContainer.RemovePropertyArgsForCall(0)
						Ω(name).Should(Equal("some-property"))
					})

					itResetsGraceTimeWhenHandling(func() {
						err := container.RemoveProperty("some-property")
						Ω(err).ShouldNot(HaveOccurred())
					})

					itFailsWhenTheContainerIsNotFound(func() error {
						return container.RemoveProperty("some-property")
					})
				})

				Context("when setting the property fails", func() {
					BeforeEach(func() {
						fakeContainer.RemovePropertyReturns(errors.New("oh no!"))
					})

					It("returns an error", func() {
						err := container.RemoveProperty("some-property")
						Ω(err).Should(HaveOccurred())
					})
				})
			})
		})

		Describe("streaming in", func() {
			It("streams the file in, waits for completion, and succeeds", func() {
				data := bytes.NewBufferString("chunk-1;chunk-2;chunk-3;")

				fakeContainer.StreamInStub = func(spec garden.StreamInSpec) error {
					Ω(spec.Path).Should(Equal("/dst/path"))
					Ω(spec.User).Should(Equal("frank"))
					Ω(ioutil.ReadAll(spec.TarStream)).Should(Equal([]byte("chunk-1;chunk-2;chunk-3;")))
					return nil
				}

				err := container.StreamIn(garden.StreamInSpec{User: "frank", Path: "/dst/path", TarStream: data})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.StreamInCallCount()).Should(Equal(1))
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.StreamIn(garden.StreamInSpec{Path: "/dst/path"})
			})

			Context("when copying in to the container fails", func() {
				BeforeEach(func() {
					fakeContainer.StreamInReturns(errors.New("oh no!"))
				})

				It("fails", func() {
					err := container.StreamIn(garden.StreamInSpec{User: "bob", Path: "/dst/path"})
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("streaming out", func() {
			var streamOut io.ReadCloser

			BeforeEach(func() {
				streamOut = ioutil.NopCloser(bytes.NewBuffer([]byte("hello-world!")))
			})

			JustBeforeEach(func() {
				fakeContainer.StreamOutReturns(streamOut, nil)
			})

			It("streams the bits out and succeeds", func() {
				reader, err := container.StreamOut(garden.StreamOutSpec{User: "frank", Path: "/src/path"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(reader).ShouldNot(BeZero())

				streamedContent, err := ioutil.ReadAll(reader)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(string(streamedContent)).Should(Equal("hello-world!"))

				Ω(fakeContainer.StreamOutArgsForCall(0)).Should(Equal(garden.StreamOutSpec{User: "frank", Path: "/src/path"}))
			})

			Context("when the connection dies as we're streaming", func() {
				var closer *closeChecker

				BeforeEach(func() {
					closer = &closeChecker{}

					streamOut = closer
				})

				It("closes the backend's stream", func() {
					reader, err := container.StreamOut(garden.StreamOutSpec{User: "frank", Path: "/src/path"})
					Ω(err).ShouldNot(HaveOccurred())

					err = reader.Close()
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(closer.Closed).Should(BeTrue())
				})
			})

			itResetsGraceTimeWhenHandling(func() {
				reader, err := container.StreamOut(garden.StreamOutSpec{User: "frank", Path: "/src/path"})
				Ω(err).ShouldNot(HaveOccurred())
				Ω(reader).ShouldNot(BeZero())

				_, err = ioutil.ReadAll(reader)
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, err := container.StreamOut(garden.StreamOutSpec{User: "frank", Path: "/src/path"})
				return err
			})

			Context("when streaming out of the container fails", func() {
				JustBeforeEach(func() {
					fakeContainer.StreamOutReturns(nil, errors.New("oh no!"))
				})

				It("returns an error", func() {
					_, err := container.StreamOut(garden.StreamOutSpec{User: "frank", Path: "/src/path"})
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("limiting bandwidth", func() {
			It("sets the container's bandwidth limits", func() {
				setLimits := garden.BandwidthLimits{
					RateInBytesPerSecond:      123,
					BurstRateInBytesPerSecond: 456,
				}

				err := container.LimitBandwidth(setLimits)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitBandwidthArgsForCall(0)).Should(Equal(setLimits))
			})

			itResetsGraceTimeWhenHandling(func() {
				err := container.LimitBandwidth(garden.BandwidthLimits{
					RateInBytesPerSecond:      123,
					BurstRateInBytesPerSecond: 456,
				})
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.LimitBandwidth(garden.BandwidthLimits{
					RateInBytesPerSecond:      123,
					BurstRateInBytesPerSecond: 456,
				})
			})

			Context("when limiting the bandwidth fails", func() {
				BeforeEach(func() {
					fakeContainer.LimitBandwidthReturns(errors.New("oh no!"))
				})

				It("fails", func() {
					err := container.LimitBandwidth(garden.BandwidthLimits{
						RateInBytesPerSecond:      123,
						BurstRateInBytesPerSecond: 456,
					})
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("getting the current bandwidth limits", func() {
			It("returns the limits returned by the backend", func() {
				effectiveLimits := garden.BandwidthLimits{
					RateInBytesPerSecond:      1230,
					BurstRateInBytesPerSecond: 4560,
				}

				fakeContainer.CurrentBandwidthLimitsReturns(effectiveLimits, nil)

				limits, err := container.CurrentBandwidthLimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits).Should(Equal(effectiveLimits))
			})

			Context("when getting the current limits fails", func() {
				BeforeEach(func() {
					fakeContainer.CurrentBandwidthLimitsReturns(garden.BandwidthLimits{}, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.CurrentBandwidthLimits()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("limiting memory", func() {
			setLimits := garden.MemoryLimits{1024}

			It("sets the container's memory limits", func() {
				err := container.LimitMemory(setLimits)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitMemoryArgsForCall(0)).Should(Equal(setLimits))
			})

			itResetsGraceTimeWhenHandling(func() {
				err := container.LimitMemory(setLimits)
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.LimitMemory(garden.MemoryLimits{123})
			})

			Context("when limiting the memory fails", func() {
				BeforeEach(func() {
					fakeContainer.LimitMemoryReturns(errors.New("oh no!"))
				})

				It("fail", func() {
					err := container.LimitMemory(garden.MemoryLimits{123})
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("getting memory limits", func() {
			It("obtains the current limits", func() {
				effectiveLimits := garden.MemoryLimits{2048}
				fakeContainer.CurrentMemoryLimitsReturns(effectiveLimits, nil)

				limits, err := container.CurrentMemoryLimits()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(limits).ShouldNot(BeZero())

				Ω(limits).Should(Equal(effectiveLimits))
			})

			It("does not change the memory limit", func() {
				_, err := container.CurrentMemoryLimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitMemoryCallCount()).Should(BeZero())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, err := container.CurrentMemoryLimits()
				return err
			})

			Context("when getting the current memory limits fails", func() {
				BeforeEach(func() {
					fakeContainer.CurrentMemoryLimitsReturns(garden.MemoryLimits{}, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.CurrentMemoryLimits()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("limiting disk", func() {
			var setLimits garden.DiskLimits

			BeforeEach(func() {
				setLimits = garden.DiskLimits{
					InodeSoft: 333,
					InodeHard: 444,

					ByteSoft: 555,
					ByteHard: 666,
				}
			})

			It("sets the container's disk limits and returns the current limits", func() {
				err := container.LimitDisk(setLimits)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitDiskArgsForCall(0)).Should(Equal(setLimits))
			})

			itResetsGraceTimeWhenHandling(func() {
				err := container.LimitDisk(setLimits)
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.LimitDisk(garden.DiskLimits{})
			})

			Context("when limiting the disk fails", func() {
				BeforeEach(func() {
					fakeContainer.LimitDiskReturns(errors.New("oh no!"))
				})

				It("fails", func() {
					err := container.LimitDisk(setLimits)
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("getting the current disk limits", func() {
			currentLimits := garden.DiskLimits{
				InodeSoft: 3333,
				InodeHard: 4444,

				ByteSoft: 5555,
				ByteHard: 6666,
			}

			It("returns the limits returned by the backend", func() {
				fakeContainer.CurrentDiskLimitsReturns(currentLimits, nil)

				limits, err := container.CurrentDiskLimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits).Should(Equal(currentLimits))
			})

			It("does not change the disk limit", func() {
				_, err := container.CurrentDiskLimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitDiskCallCount()).Should(BeZero())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, err := container.CurrentDiskLimits()
				return err
			})

			Context("when getting the current disk limits fails", func() {
				BeforeEach(func() {
					fakeContainer.CurrentDiskLimitsReturns(garden.DiskLimits{}, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.CurrentDiskLimits()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("set the cpu limit", func() {
			setLimits := garden.CPULimits{123}

			It("sets the container's CPU shares", func() {
				err := container.LimitCPU(setLimits)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitCPUArgsForCall(0)).Should(Equal(setLimits))
			})

			itResetsGraceTimeWhenHandling(func() {
				err := container.LimitCPU(setLimits)
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.LimitCPU(setLimits)
			})

			Context("when limiting the CPU fails", func() {
				BeforeEach(func() {
					fakeContainer.LimitCPUReturns(errors.New("oh no!"))
				})

				It("fails", func() {
					err := container.LimitCPU(setLimits)
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("get the current cpu limits", func() {
			effectiveLimits := garden.CPULimits{456}

			It("gets the current limits", func() {
				fakeContainer.CurrentCPULimitsReturns(effectiveLimits, nil)

				limits, err := container.CurrentCPULimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits).Should(Equal(effectiveLimits))
			})

			It("does not change the cpu limits", func() {
				_, err := container.CurrentCPULimits()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.LimitCPUCallCount()).Should(BeZero())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, err := container.CurrentCPULimits()
				return err
			})

			Context("when getting the current CPU limits fails", func() {
				BeforeEach(func() {
					fakeContainer.CurrentCPULimitsReturns(garden.CPULimits{}, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.CurrentCPULimits()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("net in", func() {
			It("maps the ports and returns them", func() {
				fakeContainer.NetInReturns(111, 222, nil)

				hostPort, containerPort, err := container.NetIn(123, 456)
				Ω(err).ShouldNot(HaveOccurred())

				hp, cp := fakeContainer.NetInArgsForCall(0)
				Ω(hp).Should(Equal(uint32(123)))
				Ω(cp).Should(Equal(uint32(456)))

				Ω(hostPort).Should(Equal(uint32(111)))
				Ω(containerPort).Should(Equal(uint32(222)))
			})

			itResetsGraceTimeWhenHandling(func() {
				_, _, err := container.NetIn(123, 456)
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, _, err := container.NetIn(123, 456)
				return err
			})

			Context("when mapping the port fails", func() {
				BeforeEach(func() {
					fakeContainer.NetInReturns(0, 0, errors.New("oh no!"))
				})

				It("fails", func() {
					_, _, err := container.NetIn(123, 456)
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("net out", func() {
			Context("when a zero-value NetOutRule is supplied", func() {
				It("permits all TCP traffic to everywhere, with logging not enabled", func() {
					Ω(container.NetOut(garden.NetOutRule{})).Should(Succeed())
					rule := fakeContainer.NetOutArgsForCall(0)

					Ω(rule.Protocol).Should(Equal(garden.ProtocolAll))
					Ω(rule.Networks).Should(BeNil())
					Ω(rule.Ports).Should(BeNil())
					Ω(rule.ICMPs).Should(BeNil())
					Ω(rule.Log).Should(Equal(false))
				})
			})

			Context("when protocol is specified", func() {
				Context("as TCP", func() {
					It("permits TCP traffic", func() {
						Ω(container.NetOut(garden.NetOutRule{
							Protocol: garden.ProtocolTCP,
						})).Should(Succeed())
						rule := fakeContainer.NetOutArgsForCall(0)
						Ω(rule.Protocol).Should(Equal(garden.ProtocolTCP))
					})
				})

				Context("as UDP", func() {
					It("permits UDP traffic", func() {
						Ω(container.NetOut(garden.NetOutRule{
							Protocol: garden.ProtocolUDP,
						})).Should(Succeed())
						rule := fakeContainer.NetOutArgsForCall(0)
						Ω(rule.Protocol).Should(Equal(garden.ProtocolUDP))
					})
				})

				Context("as ICMP", func() {
					It("permits ICMP traffic", func() {
						Ω(container.NetOut(garden.NetOutRule{
							Protocol: garden.ProtocolICMP,
						})).Should(Succeed())
						rule := fakeContainer.NetOutArgsForCall(0)
						Ω(rule.Protocol).Should(Equal(garden.ProtocolICMP))
					})
				})

				Context("as ALL", func() {
					It("permits ALL traffic", func() {
						Ω(container.NetOut(garden.NetOutRule{
							Protocol: garden.ProtocolAll,
						})).Should(Succeed())

						rule := fakeContainer.NetOutArgsForCall(0)
						Ω(rule.Protocol).Should(Equal(garden.ProtocolAll))
					})
				})
			})

			Context("when network is specified", func() {
				It("permits traffic to that network", func() {
					Ω(container.NetOut(garden.NetOutRule{
						Networks: []garden.IPRange{
							{net.ParseIP("1.3.5.7"), net.ParseIP("9.9.7.6")},
						},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.Networks).Should(Equal([]garden.IPRange{
						{
							Start: net.ParseIP("1.3.5.7"),
							End:   net.ParseIP("9.9.7.6"),
						},
					}))
				})
			})

			Context("when multiple networks are specified", func() {
				It("permits traffic to those networks", func() {
					Ω(container.NetOut(garden.NetOutRule{
						Networks: []garden.IPRange{
							{net.ParseIP("1.3.5.7"), net.ParseIP("9.9.7.6")},
							{net.ParseIP("2.4.6.8"), net.ParseIP("8.6.4.2")},
						},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.Networks).Should(ConsistOf(
						garden.IPRange{
							Start: net.ParseIP("1.3.5.7"),
							End:   net.ParseIP("9.9.7.6"),
						},
						garden.IPRange{
							Start: net.ParseIP("2.4.6.8"),
							End:   net.ParseIP("8.6.4.2"),
						},
					))
				})
			})

			Context("when ports are specified", func() {
				It("permits traffic to those ports", func() {
					Ω(container.NetOut(garden.NetOutRule{
						Ports: []garden.PortRange{
							{4, 44},
						},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.Ports).Should(Equal([]garden.PortRange{
						{
							Start: 4,
							End:   44,
						},
					}))
				})
			})

			Context("when multiple ports are specified", func() {
				It("permits traffic to those ports", func() {
					Ω(container.NetOut(garden.NetOutRule{
						Ports: []garden.PortRange{
							{4, 44},
							{563, 3944},
						},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.Ports).Should(ConsistOf(
						garden.PortRange{
							Start: 4,
							End:   44,
						},
						garden.PortRange{
							Start: 563,
							End:   3944,
						},
					))
				})
			})

			Context("when icmps are specified without a code", func() {
				It("permits traffic matching those icmps", func() {
					Ω(container.NetOut(garden.NetOutRule{
						ICMPs: &garden.ICMPControl{Type: 4},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.ICMPs).Should(Equal(&garden.ICMPControl{Type: 4}))
				})
			})

			Context("when icmps are specified with a code", func() {
				It("permits traffic matching those icmps", func() {
					var code garden.ICMPCode = 34
					Ω(container.NetOut(garden.NetOutRule{
						ICMPs: &garden.ICMPControl{Type: 4, Code: &code},
					})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.ICMPs).Should(Equal(&garden.ICMPControl{Type: 4, Code: &code}))
				})
			})

			Context("when log is true", func() {
				It("requests that the rule logs", func() {
					Ω(container.NetOut(garden.NetOutRule{Log: true})).Should(Succeed())

					rule := fakeContainer.NetOutArgsForCall(0)
					Ω(rule.Log).Should(BeTrue())
				})
			})

			itResetsGraceTimeWhenHandling(func() {
				err := container.NetOut(garden.NetOutRule{})
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				return container.NetOut(garden.NetOutRule{})
			})

			Context("when permitting traffic fails", func() {
				BeforeEach(func() {
					fakeContainer.NetOutReturns(errors.New("oh no!"))
				})

				It("fails", func() {
					err := container.NetOut(garden.NetOutRule{})
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("info", func() {
			containerInfo := garden.ContainerInfo{
				State:         "active",
				Events:        []string{"oom", "party"},
				HostIP:        "host-ip",
				ContainerIP:   "container-ip",
				ExternalIP:    "external-ip",
				ContainerPath: "/path/to/container",
				ProcessIDs:    []uint32{1, 2},
				Properties: garden.Properties{
					"foo": "bar",
					"a":   "b",
				},
				MappedPorts: []garden.PortMapping{
					{HostPort: 1234, ContainerPort: 5678},
					{HostPort: 1235, ContainerPort: 5679},
				},
			}

			It("reports information about the container", func() {
				fakeContainer.InfoReturns(containerInfo, nil)

				info, err := container.Info()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(info).Should(Equal(containerInfo))
			})

			itResetsGraceTimeWhenHandling(func() {
				_, err := container.Info()
				Ω(err).ShouldNot(HaveOccurred())
			})

			itFailsWhenTheContainerIsNotFound(func() error {
				_, err := container.Info()
				return err
			})

			Context("when getting container info fails", func() {
				BeforeEach(func() {
					fakeContainer.InfoReturns(garden.ContainerInfo{}, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.Info()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Describe("BulkInfo", func() {

			handles := []string{"handle1", "handle2"}

			expectedBulkInfo := map[string]garden.ContainerInfoEntry{
				"handle1": garden.ContainerInfoEntry{
					Info: garden.ContainerInfo{
						State: "container1state",
					},
				},
				"handle2": garden.ContainerInfoEntry{
					Info: garden.ContainerInfo{
						State: "container2state",
					},
				},
			}

			It("reports information about containers by list of handles", func() {
				serverBackend.BulkInfoReturns(expectedBulkInfo, nil)

				bulkInfo, err := apiClient.BulkInfo(handles)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(bulkInfo).To(Equal(expectedBulkInfo))
			})

			Context("when retrieving bulk info fails", func() {
				It("returns the error", func() {
					serverBackend.BulkInfoReturns(
						make(map[string]garden.ContainerInfoEntry),
						errors.New("Oh noes!"),
					)

					_, err := apiClient.BulkInfo(handles)
					Ω(err).Should(MatchError("Oh noes!"))
				})
			})

			Context("when a container is in error state", func() {
				It("return single container error", func() {

					expectedBulkInfo := map[string]garden.ContainerInfoEntry{
						"error": garden.ContainerInfoEntry{
							Err: garden.NewError("Oopps!"),
						},
						"success": garden.ContainerInfoEntry{
							Info: garden.ContainerInfo{
								State: "container2state",
							},
						},
					}

					serverBackend.BulkInfoReturns(expectedBulkInfo, nil)

					bulkInfo, err := apiClient.BulkInfo(handles)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(bulkInfo).To(Equal(expectedBulkInfo))
				})
			})
		})

		Describe("BulkMetrics", func() {

			handles := []string{"handle1", "handle2"}

			expectedBulkMetrics := map[string]garden.ContainerMetricsEntry{
				"handle1": garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						DiskStat: garden.ContainerDiskStat{
							TotalInodesUsed: 1,
						},
					},
				},
				"handle2": garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						DiskStat: garden.ContainerDiskStat{
							TotalInodesUsed: 2,
						},
					},
				},
			}

			It("reports information about containers by list of handles", func() {
				serverBackend.BulkMetricsReturns(expectedBulkMetrics, nil)

				bulkMetrics, err := apiClient.BulkMetrics(handles)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(bulkMetrics).To(Equal(expectedBulkMetrics))
			})

			Context("when retrieving bulk info fails", func() {
				It("returns the error", func() {
					serverBackend.BulkMetricsReturns(
						make(map[string]garden.ContainerMetricsEntry),
						errors.New("Oh noes!"),
					)

					_, err := apiClient.BulkMetrics(handles)
					Ω(err).Should(MatchError("Oh noes!"))
				})
			})

			Context("when a container has an error", func() {
				It("returns the error for the container", func() {

					errorBulkMetrics := map[string]garden.ContainerMetricsEntry{
						"error": garden.ContainerMetricsEntry{
							Err: garden.NewError("Oh noes!"),
						},
						"success": garden.ContainerMetricsEntry{
							Metrics: garden.Metrics{
								DiskStat: garden.ContainerDiskStat{
									TotalInodesUsed: 1,
								},
							},
						},
					}

					serverBackend.BulkMetricsReturns(errorBulkMetrics, nil)

					bulkMetrics, err := apiClient.BulkMetrics(handles)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(bulkMetrics).To(Equal(errorBulkMetrics))
				})
			})
		})

		Describe("attaching", func() {
			Context("when attaching succeeds", func() {
				BeforeEach(func() {
					fakeContainer.AttachStub = func(processID uint32, io garden.ProcessIO) (garden.Process, error) {
						writing := new(sync.WaitGroup)
						writing.Add(1)

						go func() {
							defer writing.Done()
							defer GinkgoRecover()

							_, err := fmt.Fprintf(io.Stdout, "stdout data")
							Ω(err).ShouldNot(HaveOccurred())

							in, err := ioutil.ReadAll(io.Stdin)
							Ω(err).ShouldNot(HaveOccurred())

							_, err = fmt.Fprintf(io.Stdout, "mirrored %s", string(in))
							Ω(err).ShouldNot(HaveOccurred())

							_, err = fmt.Fprintf(io.Stderr, "stderr data")
							Ω(err).ShouldNot(HaveOccurred())
						}()

						process := new(fakes.FakeProcess)

						process.IDReturns(42)

						process.WaitStub = func() (int, error) {
							writing.Wait()
							return 123, nil
						}

						return process, nil
					}
				})

				It("responds with a ProcessPayload for every chunk", func() {
					stdout := gbytes.NewBuffer()
					stderr := gbytes.NewBuffer()

					processIO := garden.ProcessIO{
						Stdin:  bytes.NewBufferString("stdin data"),
						Stdout: stdout,
						Stderr: stderr,
					}

					process, err := container.Attach(42, processIO)
					Ω(err).ShouldNot(HaveOccurred())

					pid, _ := fakeContainer.AttachArgsForCall(0)
					Ω(pid).Should(Equal(uint32(42)))

					Eventually(stdout).Should(gbytes.Say("stdout data"))
					Eventually(stdout).Should(gbytes.Say("mirrored stdin data"))
					Eventually(stderr).Should(gbytes.Say("stderr data"))

					status, err := process.Wait()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(status).Should(Equal(123))
				})

				itResetsGraceTimeWhenHandling(func() {
					process, err := container.Attach(42, garden.ProcessIO{
						Stdin: bytes.NewBufferString("hello"),
					})
					Ω(err).ShouldNot(HaveOccurred())

					status, err := process.Wait()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(status).Should(Equal(123))
				})
			})

			Context("when the container is not found", func() {
				It("fails", func() {
					serverBackend.LookupReturns(nil, errors.New("not found"))
					_, err := container.Attach(123, garden.ProcessIO{})
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when waiting on the process fails server-side", func() {
				BeforeEach(func() {
					fakeContainer.AttachStub = func(id uint32, io garden.ProcessIO) (garden.Process, error) {
						process := new(fakes.FakeProcess)

						process.IDReturns(42)
						process.WaitReturns(0, errors.New("oh no!"))

						return process, nil
					}
				})

				It("bubbles the error up", func() {
					process, err := container.Attach(42, garden.ProcessIO{})
					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})

			Context("when attaching fails", func() {
				BeforeEach(func() {
					fakeContainer.AttachReturns(nil, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.Attach(123, garden.ProcessIO{})
					Ω(err).Should(HaveOccurred())
				})

				It("closes the stdin writer", func(done Done) {
					container.Attach(123, garden.ProcessIO{})

					_, processIO := fakeContainer.AttachArgsForCall(0)
					_, err := processIO.Stdin.Read([]byte{})
					Ω(err).Should(Equal(io.EOF))

					close(done)
				})
			})
		})

		Describe("running", func() {
			processSpec := garden.ProcessSpec{
				Path: "/some/script",
				Args: []string{"arg1", "arg2"},
				Dir:  "/some/dir",
				Env: []string{
					"FLAVOR=chocolate",
					"TOPPINGS=sprinkles",
				},
				User: "root",
				Limits: garden.ResourceLimits{
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
				TTY: &garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: 80,
						Rows:    24,
					},
				},
			}

			Context("when running succeeds", func() {
				BeforeEach(func() {
					fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
						writing := new(sync.WaitGroup)
						writing.Add(1)

						go func() {
							defer writing.Done()
							defer GinkgoRecover()

							_, err := fmt.Fprintf(io.Stdout, "stdout data")
							Ω(err).ShouldNot(HaveOccurred())

							in, err := ioutil.ReadAll(io.Stdin)
							Ω(err).ShouldNot(HaveOccurred())

							_, err = fmt.Fprintf(io.Stdout, "mirrored %s", string(in))
							Ω(err).ShouldNot(HaveOccurred())

							_, err = fmt.Fprintf(io.Stderr, "stderr data")
							Ω(err).ShouldNot(HaveOccurred())
						}()

						process := new(fakes.FakeProcess)

						process.IDReturns(42)

						process.WaitStub = func() (int, error) {
							writing.Wait()
							return 123, nil
						}

						return process, nil
					}
				})

				It("should not log any environment variables and command line args", func() {
					process, err := container.Run(garden.ProcessSpec{
						User: "alice",
						Path: "echo",
						Args: []string{"-username", "banana"},
						Env:  []string{"PASSWORD=MY_SECRET"},
					}, garden.ProcessIO{
						Stdin:  bytes.NewBufferString("stdin data"),
						Stdout: GinkgoWriter,
						Stderr: GinkgoWriter,
					})
					Expect(err).ToNot(HaveOccurred())

					exitCode, err := process.Wait()
					Expect(exitCode).To(Equal(123))
					Expect(err).ToNot(HaveOccurred())

					buffer := sink.Buffer()

					Expect(buffer).ToNot(gbytes.Say("PASSWORD"))
					Expect(buffer).ToNot(gbytes.Say("MY_SECRET"))
					Expect(buffer).ToNot(gbytes.Say("-username"))
					Expect(buffer).ToNot(gbytes.Say("banana"))
				})

				It("runs the process and streams the output", func(done Done) {
					stdout := gbytes.NewBuffer()
					stderr := gbytes.NewBuffer()

					processIO := garden.ProcessIO{
						Stdin:  bytes.NewBufferString("stdin data"),
						Stdout: stdout,
						Stderr: stderr,
					}

					process, err := container.Run(processSpec, processIO)
					Ω(err).ShouldNot(HaveOccurred())

					ranSpec, processIO := fakeContainer.RunArgsForCall(0)
					Ω(ranSpec).Should(Equal(processSpec))

					Eventually(stdout).Should(gbytes.Say("stdout data"))
					Eventually(stdout).Should(gbytes.Say("mirrored stdin data"))
					Eventually(stderr).Should(gbytes.Say("stderr data"))

					status, err := process.Wait()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(status).Should(Equal(123))

					_, err = processIO.Stdin.Read([]byte{})
					Ω(err).Should(Equal(io.EOF))

					close(done)
				})

				itResetsGraceTimeWhenHandling(func() {
					process, err := container.Run(processSpec, garden.ProcessIO{
						Stdin: bytes.NewBufferString("hello"),
					})
					Ω(err).ShouldNot(HaveOccurred())

					status, err := process.Wait()
					Ω(err).ShouldNot(HaveOccurred())
					Ω(status).Should(Equal(123))
				})
			})

			Context("when the backend returns an error", func() {

				It("returns the error message", func() {
					fakeContainer.RunReturns(nil, errors.New("o no!"))
					_, err := container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
					Expect(err).To(MatchError(ContainSubstring("o no!")))
				})
			})

			Describe("when the server is shut down while there is a process running", func() {
				BeforeEach(func() {
					fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
						writing := new(sync.WaitGroup)
						writing.Add(1)

						go func() {
							defer writing.Done()
							defer GinkgoRecover()

							for {
								_, err := fmt.Fprintf(io.Stdout, "stdout data")
								Ω(err).ShouldNot(HaveOccurred())
							}
						}()

						process := new(fakes.FakeProcess)
						process.IDReturns(42)

						process.WaitStub = func() (int, error) {
							select {}
							return 0, nil
						}

						return process, nil
					}
				})

				It("does not close the process's stdin", func() {
					pipeR, _ := io.Pipe()
					_, err := container.Run(processSpec, garden.ProcessIO{Stdin: pipeR})
					Ω(err).ShouldNot(HaveOccurred())

					apiServer.Stop()
					isRunning = false

					_, processIO := fakeContainer.RunArgsForCall(0)

					readExited := make(chan struct{})
					go func() {
						_, err = ioutil.ReadAll(processIO.Stdin)
						close(readExited)
					}()

					Eventually(readExited).Should(BeClosed())
					Ω(err).Should(HaveOccurred())
					Ω(err).ShouldNot(Equal(io.EOF))
				})
			})

			Context("when the process is killed", func() {
				var fakeProcess *fakes.FakeProcess

				BeforeEach(func() {
					fakeProcess = new(fakes.FakeProcess)
					fakeProcess.IDReturns(42)
					fakeProcess.WaitStub = func() (int, error) {
						select {}
						return 0, nil
					}

					fakeContainer.RunReturns(fakeProcess, nil)
				})

				It("is eventually killed in the backend", func() {
					process, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).ShouldNot(HaveOccurred())

					err = process.Signal(garden.SignalKill)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(fakeProcess.SignalCallCount).Should(Equal(1))
					Ω(fakeProcess.SignalArgsForCall(0)).Should(Equal(garden.SignalKill))
				})
			})

			Context("when the process is terminated", func() {
				var fakeProcess *fakes.FakeProcess

				BeforeEach(func() {
					fakeProcess = new(fakes.FakeProcess)
					fakeProcess.IDReturns(42)
					fakeProcess.WaitStub = func() (int, error) {
						select {}
						return 0, nil
					}

					fakeContainer.RunReturns(fakeProcess, nil)
				})

				It("is eventually terminated in the backend", func() {
					process, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).ShouldNot(HaveOccurred())

					err = process.Signal(garden.SignalTerminate)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(fakeProcess.SignalCallCount).Should(Equal(1))
					Ω(fakeProcess.SignalArgsForCall(0)).Should(Equal(garden.SignalTerminate))
				})
			})

			Context("when the process's window size is set", func() {
				var fakeProcess *fakes.FakeProcess

				BeforeEach(func() {
					fakeProcess = new(fakes.FakeProcess)
					fakeProcess.IDReturns(42)
					fakeProcess.WaitStub = func() (int, error) {
						select {}
						return 0, nil
					}

					fakeContainer.RunReturns(fakeProcess, nil)
				})

				It("is eventually set in the backend", func() {
					process, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).ShouldNot(HaveOccurred())

					ttySpec := garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 80,
							Rows:    24,
						},
					}

					err = process.SetTTY(ttySpec)
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(fakeProcess.SetTTYCallCount).Should(Equal(1))

					Ω(fakeProcess.SetTTYArgsForCall(0)).Should(Equal(ttySpec))
				})
			})

			Context("when waiting on the process fails server-side", func() {
				BeforeEach(func() {
					fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
						process := new(fakes.FakeProcess)

						process.IDReturns(42)
						process.WaitReturns(0, errors.New("oh no!"))

						return process, nil
					}
				})

				It("bubbles the error up", func() {
					process, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})

			Context("when the container is not found", func() {
				It("fails", func() {
					serverBackend.LookupReturns(nil, errors.New("not found"))
					_, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).Should(HaveOccurred())
				})
			})

			Context("when running fails", func() {
				BeforeEach(func() {
					fakeContainer.RunReturns(nil, errors.New("oh no!"))
				})

				It("fails", func() {
					_, err := container.Run(processSpec, garden.ProcessIO{})
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})
})

type closeChecker struct {
	closed bool
	sync.Mutex
}

func (checker *closeChecker) Read(b []byte) (int, error) {
	b[0] = 'x'
	return 1, nil
}

func (checker *closeChecker) Close() error {
	checker.Lock()
	checker.closed = true
	checker.Unlock()
	return nil
}

func (checker *closeChecker) Closed() bool {
	checker.Lock()
	defer checker.Unlock()
	return checker.closed
}
