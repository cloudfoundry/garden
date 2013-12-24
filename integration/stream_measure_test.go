package integration_test

import (
	"fmt"
	"net"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/gordon"
)

var _ = Describe("The Warden server", func() {
	var wardenClient *warden.Client

	BeforeEach(func() {
		socketPath := os.Getenv("WARDEN_TEST_SOCKET")
		Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccurred())

		wardenClient = warden.NewClient(&warden.ConnectionInfo{SocketPath: socketPath})

		err := wardenClient.Connect()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("streaming output from a chatty job", func() {
		var handle string

		BeforeEach(func() {
			res, err := wardenClient.Create()
			Expect(err).ToNot(HaveOccurred())

			handle = res.GetHandle()
		})

		for i := 0; i <= 100; i += 10 {
			Context(fmt.Sprintf("with %d streams", i), func() {
				var started time.Time
				var receivedBytes uint64

				numToSpawn := i

				BeforeEach(func() {
					receivedBytes = 0
					started = time.Now()

					for j := 0; j < numToSpawn; j++ {
						spawnRes, err := wardenClient.Spawn(
							handle,
							"while true; do echo hi out; echo hi err 1>&2; done",
							true,
						)
						Expect(err).ToNot(HaveOccurred())

						results, err := wardenClient.Stream(handle, spawnRes.GetJobId())
						Expect(err).ToNot(HaveOccurred())

						go func(results chan *warden.StreamResponse) {
							for {
								res, ok := <-results
								if !ok {
									break
								}

								receivedBytes += uint64(len(res.GetData()))

								continue

								fmt.Println(res)
							}
						}(results)
					}
				})

				AfterEach(func() {
					_, err := wardenClient.Destroy(handle)
					Expect(err).ToNot(HaveOccurred())
				})

				Measure("it should not adversely affect the rest of the API", func(b Benchmarker) {
					var newHandle string

					b.Time("creating another container", func() {
						res, err := wardenClient.Create()
						Expect(err).ToNot(HaveOccurred())

						newHandle = res.GetHandle()
					})

					for i := 0; i < 10; i++ {
						b.Time("getting container info (10x)", func() {
							_, err := wardenClient.Info(newHandle)
							Expect(err).ToNot(HaveOccurred())
						})
					}

					for i := 0; i < 10; i++ {
						b.Time("running a job (10x)", func() {
							wardenClient.Run(newHandle, "ls")
							//Expect(err).ToNot(HaveOccurred())
						})
					}

					b.Time("destroying the container", func() {
						_, err := wardenClient.Destroy(newHandle)
						Expect(err).ToNot(HaveOccurred())
					})

					b.RecordValue(
						"received rate (bytes/second)",
						float64(receivedBytes) / float64(time.Since(started) / time.Second),
					)
				}, 5)
			})
		}
	})
})

func ErrorDialingUnix(socketPath string) func() error {
	return func() error {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
		}

		return err
	}
}
