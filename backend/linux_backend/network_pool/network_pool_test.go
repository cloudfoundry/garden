package network_pool_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend/linux_backend/network_pool"
)

var _ = Describe("Network Pool", func() {
	var pool *network_pool.NetworkPool

	BeforeEach(func() {
		_, ipNet, err := net.ParseCIDR("10.244.0.0/22")
		Expect(err).ToNot(HaveOccured())

		pool = network_pool.New(ipNet)
	})

	Describe("acquiring", func() {
		It("takes the next /30 in the pool", func() {
			network1, err := pool.Acquire()
			Expect(err).ToNot(HaveOccured())

			Expect(network1.Equal(net.IPv4(10, 244, 0, 0))).To(BeTrue())

			network2, err := pool.Acquire()
			Expect(err).ToNot(HaveOccured())

			Expect(network2.Equal(net.IPv4(10, 244, 0, 4))).To(BeTrue())
		})

		Context("when the pool is exhausted", func() {
			It("returns an error", func() {
				for i := 0; i < 256; i++ {
					_, err := pool.Acquire()
					Expect(err).ToNot(HaveOccured())
				}

				_, err := pool.Acquire()
				Expect(err).To(HaveOccured())
			})
		})
	})

	Describe("releasing", func() {
		It("places a network back and the end of the pool", func() {
			first, err := pool.Acquire()
			Expect(err).ToNot(HaveOccured())

			pool.Release(first)

			for i := 0; i < 255; i++ {
				_, err := pool.Acquire()
				Expect(err).ToNot(HaveOccured())
			}

			last, err := pool.Acquire()
			Expect(err).ToNot(HaveOccured())
			Expect(last).To(Equal(first))
		})

		Context("when the released network is out of the range", func() {
			It("does not add it to the pool", func() {
				for i := 0; i < 256; i++ {
					_, err := pool.Acquire()
					Expect(err).ToNot(HaveOccured())
				}

				pool.Release(net.ParseIP("127.0.0.1"))

				_, err := pool.Acquire()
				Expect(err).To(HaveOccured())
			})
		})
	})
})
