package lifecycle_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Through a restart", func() {
	var handle string

	BeforeEach(func() {
		res, err := client.Create()
		Expect(err).ToNot(HaveOccurred())

		handle = res.GetHandle()
	})

	AfterEach(func() {
		err := runner.Stop()
		Expect(err).ToNot(HaveOccurred())

		err = runner.DestroyContainers()
		Expect(err).ToNot(HaveOccurred())

		err = runner.Start()
		Expect(err).ToNot(HaveOccurred())
	})

	restartServer := func() {
		err := runner.Stop()
		Expect(err).ToNot(HaveOccurred())

		err = runner.Start()
		Expect(err).ToNot(HaveOccurred())
	}

	It("retains the container list", func() {
		restartServer()

		res, err := client.List()
		Expect(err).ToNot(HaveOccurred())

		Expect(res.GetHandles()).To(ContainElement(handle))
	})

	PDescribe("a started job", func() {
		It("continues to stream", func(done Done) {
			res, err := client.Spawn(handle, "while true; do echo hi; sleep 0.5; done", false)
			Expect(err).ToNot(HaveOccurred())

			jobID := res.GetJobId()

			restartServer()

			stream, err := client.Stream(handle, jobID)
			Expect(err).ToNot(HaveOccurred())

			Expect((<-stream).GetData()).To(Equal("hi\n"))

			close(done)
		}, 10.0)

		It("does not have its job ID repeated", func() {
			res, err := client.Spawn(handle, "while true; do echo hi; sleep 0.5; done", false)
			Expect(err).ToNot(HaveOccurred())

			jobID1 := res.GetJobId()

			restartServer()

			res, err = client.Spawn(handle, "while true; do echo hi; sleep 0.5; done", false)
			Expect(err).ToNot(HaveOccurred())

			jobID2 := res.GetJobId()

			Expect(jobID1).ToNot(Equal(jobID2))
		})

		Context("with output discarded", func() {
			It("continues to not collect output", func(done Done) {
				res, err := client.Spawn(handle, "while true; do echo hi; sleep 0.5; done", true)
				Expect(err).ToNot(HaveOccurred())

				jobID := res.GetJobId()

				restartServer()

				go func() {
					res, err := client.Link(handle, jobID)
					Expect(err).ToNot(HaveOccurred())

					Expect(res.GetStdout()).To(BeEmpty())
					Expect(res.GetStderr()).To(BeEmpty())

					close(done)
				}()

				time.Sleep(100 * time.Millisecond)

				// stop container to kill running job
				_, err = client.Stop(handle, false, false)
				Expect(err).ToNot(HaveOccurred())
			}, 10.0)
		})
	})

	Describe("a memory limit", func() {
		It("is still enforced", func() {
			_, err := client.LimitMemory(handle, 32*1024*1024)
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			res, err := client.Run(handle, "exec ruby -e '$stdout.sync = true; puts :hello; puts (\"x\" * 64 * 1024 * 1024).size; puts :goodbye; exit 42'")
			Expect(err).ToNot(HaveOccurred())

			// cgroups OOM killer seems to leave no trace of the process;
			// there's no exit status indicator, so just assert that the one
			// we tried to exit with after over-allocating is not seen
			Expect(res.GetStdout()).To(Equal("hello\n"))
			Expect(res.GetExitStatus()).ToNot(Equal(uint32(42)))
		})
	})

	PDescribe("a container's active job", func() {
		It("is still tracked", func() {
			res, err := client.Spawn(handle, "while true; do echo hi; sleep 0.5; done", true)
			Expect(err).ToNot(HaveOccurred())

			jobID := res.GetJobId()

			restartServer()

			info, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetJobIds()).To(ContainElement(uint64(jobID)))
		})
	})

	Describe("a container's list of events", func() {
		It("is still reported", func() {
			_, err := client.LimitMemory(handle, 32*1024*1024)
			Expect(err).ToNot(HaveOccurred())

			// trigger 'out of memory' event
			_, err = client.Run(handle, "exec ruby -e '$stdout.sync = true; puts :hello; puts (\"x\" * 64 * 1024 * 1024).size; puts :goodbye; exit 42'")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() []string {
				info, err := client.Info(handle)
				Expect(err).ToNot(HaveOccurred())

				return info.GetEvents()
			}).Should(ContainElement("out of memory"))

			restartServer()

			info, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetEvents()).To(ContainElement("out of memory"))
		})
	})

	Describe("a container's state", func() {
		It("is still reported", func() {
			info, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetState()).To(Equal("active"))

			restartServer()

			info, err = client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetState()).To(Equal("active"))

			_, err = client.Stop(handle, false, false)
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			info, err = client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetState()).To(Equal("stopped"))
		})
	})

	Describe("a container's network", func() {
		It("does not get reused", func() {
			infoA, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			res, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			infoB, err := client.Info(res.GetHandle())
			Expect(err).ToNot(HaveOccurred())

			Expect(infoA.GetHostIp()).ToNot(Equal(infoB.GetHostIp()))
			Expect(infoA.GetContainerIp()).ToNot(Equal(infoB.GetContainerIp()))
		})
	})

	Describe("a container's mapped port", func() {
		It("does not get reused", func() {
			netInA, err := client.NetIn(handle)
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			createRes, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			netInB, err := client.NetIn(createRes.GetHandle())
			Expect(err).ToNot(HaveOccurred())

			Expect(netInA.GetHostPort()).ToNot(Equal(netInB.GetHostPort()))
			Expect(netInA.GetContainerPort()).ToNot(Equal(netInB.GetContainerPort()))
		})
	})

	Describe("a container's user", func() {
		It("does not get reused", func() {
			idResA, err := client.Run(handle, "id -u")
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			createRes, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			idResB, err := client.Run(createRes.GetHandle(), "id -u")
			Expect(err).ToNot(HaveOccurred())

			fmt.Println("A", "B", idResA, idResB)

			Expect(idResB.GetStdout()).ToNot(Equal(idResA.GetStdout()))
		})
	})
})
