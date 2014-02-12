package measurements_test

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/gordon/warden"
)

var _ = Describe("The Warden server", func() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	Describe("streaming output from a chatty job", func() {
		var handle string

		BeforeEach(func() {
			res, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			handle = res.GetHandle()
		})

		streamCounts := []int{0}

		for i := 1; i <= 128; i *= 2 {
			streamCounts = append(streamCounts, i)
		}

		for _, streams := range streamCounts {
			Context(fmt.Sprintf("with %d streams", streams), func() {
				var started time.Time
				var receivedBytes uint64

				numToSpawn := streams

				BeforeEach(func() {
					receivedBytes = 0
					started = time.Now()

					spawned := make(chan bool)

					for j := 0; j < numToSpawn; j++ {
						go func() {
							_, results, err := client.Run(
								handle,
								"cat /dev/zero",
							)
							Expect(err).ToNot(HaveOccurred())

							go func(results <-chan *warden.ProcessPayload) {
								for {
									res, ok := <-results
									if !ok {
										break
									}

									atomic.AddUint64(&receivedBytes, uint64(len(res.GetData())))
								}
							}(results)

							spawned <- true
						}()
					}

					for j := 0; j < numToSpawn; j++ {
						<-spawned
					}
				})

				AfterEach(func() {
					_, err := client.Destroy(handle)
					Expect(err).ToNot(HaveOccurred())
				})

				Measure("it should not adversely affect the rest of the API", func(b Benchmarker) {
					var newHandle string

					b.Time("creating another container", func() {
						res, err := client.Create()
						Expect(err).ToNot(HaveOccurred())

						newHandle = res.GetHandle()
					})

					for i := 0; i < 10; i++ {
						b.Time("getting container info (10x)", func() {
							_, err := client.Info(newHandle)
							Expect(err).ToNot(HaveOccurred())
						})
					}

					for i := 0; i < 10; i++ {
						b.Time("running a job (10x)", func() {
							_, stream, err := client.Run(newHandle, "ls")
							Expect(err).ToNot(HaveOccurred())

							for _ = range stream {

							}
						})
					}

					b.Time("destroying the container", func() {
						_, err := client.Destroy(newHandle)
						Expect(err).ToNot(HaveOccurred())
					})

					b.RecordValue(
						"received rate (bytes/second)",
						float64(receivedBytes)/float64(time.Since(started)/time.Second),
					)

					fmt.Println("total time:", time.Since(started))
				}, 5)
			})
		}
	})
})
