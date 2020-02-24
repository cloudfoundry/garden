package server_test

import (
	"io"

	. "code.cloudfoundry.org/garden/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Chanwriter", func() {
	var (
		channel    chan []byte
		chanWriter io.WriteCloser
	)

	BeforeEach(func() {
		channel = make(chan []byte, 1)
		chanWriter = NewChanWriter(channel)
	})

	It("writes the data to the channel", func() {
		n, err := chanWriter.Write([]byte("potato"))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len("potato")))
	})

	When("the channel is full", func() {
		BeforeEach(func() {
			channel <- []byte("your write will block now")
		})

		It("does not block", func(done Done) {
			_, err := chanWriter.Write([]byte("potato"))
			Expect(err).NotTo(HaveOccurred())
			close(done)
		}, 3.0)

		// We are OK with the fact that we will drop data if the clients are too slow to read it.
		// People should still treat it as a successful write though.
		It("it still returns the size of the data", func(done Done) {
			n, err := chanWriter.Write([]byte("potato"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(len("potato")))
			close(done)
		}, 3.0)
	})
})
