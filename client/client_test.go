package client_test

import (
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fake_connection"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var _ = Describe("Client", func() {
	var client Client

	var fakeConnection *fake_connection.FakeConnection

	BeforeEach(func() {
		fakeConnection = fake_connection.New()
	})

	JustBeforeEach(func() {
		client = New(fakeConnection)
	})

	Describe("Capacity", func() {
		BeforeEach(func() {
			fakeConnection.WhenGettingCapacity = func() (warden.Capacity, error) {
				return warden.Capacity{
					MemoryInBytes: 1111,
					DiskInBytes:   2222,
					MaxContainers: 42,
				}, nil
			}
		})

		It("sends a capacity request and returns the capacity", func() {
			capacity, err := client.Capacity()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(capacity.MemoryInBytes).Should(Equal(uint64(1111)))
			Ω(capacity.DiskInBytes).Should(Equal(uint64(2222)))
		})

		Context("when getting capacity fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenGettingCapacity = func() (warden.Capacity, error) {
					return warden.Capacity{}, disaster
				}
			})

			It("returns the error", func() {
				_, err := client.Capacity()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Create", func() {
		It("sends a create request and returns a container", func() {
			spec := warden.ContainerSpec{
				RootFSPath: "/some/roofs",
			}

			fakeConnection.WhenCreating = func(spec warden.ContainerSpec) (string, error) {
				return "some-handle", nil
			}

			container, err := client.Create(spec)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(container).ShouldNot(BeNil())

			Ω(fakeConnection.Created()).Should(ContainElement(spec))
			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenCreating = func(spec warden.ContainerSpec) (string, error) {
					return "", disaster
				}
			})

			It("returns it", func() {
				_, err := client.Create(warden.ContainerSpec{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Containers", func() {
		It("sends a list request and returns all containers", func() {
			fakeConnection.WhenListing = func(warden.Properties) ([]string, error) {
				return []string{"handle-a", "handle-b"}, nil
			}

			props := warden.Properties{"foo": "bar"}

			containers, err := client.Containers(props)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.ListedProperties()).Should(ContainElement(props))

			Ω(containers).Should(HaveLen(2))
			Ω(containers[0].Handle()).Should(Equal("handle-a"))
			Ω(containers[1].Handle()).Should(Equal("handle-b"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenListing = func(warden.Properties) ([]string, error) {
					return nil, disaster
				}
			})

			It("returns it", func() {
				_, err := client.Containers(nil)
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Destroy", func() {
		It("sends a destroy request", func() {
			err := client.Destroy("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.Destroyed()).Should(ContainElement("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenDestroying = func(string) error {
					return disaster
				}
			})

			It("returns it", func() {
				err := client.Destroy("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Lookup", func() {
		It("sends a list request", func() {
			fakeConnection.WhenListing = func(warden.Properties) ([]string, error) {
				return []string{"some-handle", "some-other-handle"}, nil
			}

			container, err := client.Lookup("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeConnection.WhenListing = func(warden.Properties) ([]string, error) {
					return []string{"some-other-handle"}, nil
				}
			})

			It("returns an error", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(HaveOccurred())
				Ω(err.Error()).Should(Equal("container not found: some-handle"))
			})
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.WhenListing = func(warden.Properties) ([]string, error) {
					return nil, disaster
				}
			})

			It("returns it", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
