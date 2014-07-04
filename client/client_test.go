package client_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection/fakes"
	"github.com/cloudfoundry-incubator/garden/warden"
)

var _ = Describe("Client", func() {
	var client Client

	var fakeConnection *fakes.FakeConnection

	BeforeEach(func() {
		fakeConnection = new(fakes.FakeConnection)
	})

	JustBeforeEach(func() {
		client = New(fakeConnection)
	})

	Describe("Capacity", func() {
		BeforeEach(func() {
			fakeConnection.CapacityReturns(
				warden.Capacity{
					MemoryInBytes: 1111,
					DiskInBytes:   2222,
					MaxContainers: 42,
				},
				nil,
			)
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
				fakeConnection.CapacityReturns(warden.Capacity{}, disaster)
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

			fakeConnection.CreateReturns("some-handle", nil)

			container, err := client.Create(spec)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(container).ShouldNot(BeNil())

			Ω(fakeConnection.CreateArgsForCall(0)).Should(Equal(spec))

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.CreateReturns("", disaster)
			})

			It("returns it", func() {
				_, err := client.Create(warden.ContainerSpec{})
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Containers", func() {
		It("sends a list request and returns all containers", func() {
			fakeConnection.ListReturns([]string{"handle-a", "handle-b"}, nil)

			props := warden.Properties{"foo": "bar"}

			containers, err := client.Containers(props)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeConnection.ListArgsForCall(0)).Should(Equal(props))

			Ω(containers).Should(HaveLen(2))
			Ω(containers[0].Handle()).Should(Equal("handle-a"))
			Ω(containers[1].Handle()).Should(Equal("handle-b"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.ListReturns(nil, disaster)
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

			Ω(fakeConnection.DestroyArgsForCall(0)).Should(Equal("some-handle"))
		})

		Context("when there is a connection error", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeConnection.DestroyReturns(disaster)
			})

			It("returns it", func() {
				err := client.Destroy("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Lookup", func() {
		It("sends a list request", func() {
			fakeConnection.ListReturns([]string{"some-handle", "some-other-handle"}, nil)

			container, err := client.Lookup("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(container.Handle()).Should(Equal("some-handle"))
		})

		Context("when the container is not found", func() {
			BeforeEach(func() {
				fakeConnection.ListReturns([]string{"some-other-handle"}, nil)
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
				fakeConnection.ListReturns(nil, disaster)
			})

			It("returns it", func() {
				_, err := client.Lookup("some-handle")
				Ω(err).Should(Equal(disaster))
			})
		})
	})
})
