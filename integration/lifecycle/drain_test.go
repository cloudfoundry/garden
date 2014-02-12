package lifecycle_test

import (
	"bytes"
	"fmt"
	"io"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/gordon/warden"
)

func readUntilExit(stream <-chan *warden.ProcessPayload) (string, string, uint32) {
	stdout := ""
	stderr := ""
	exitStatus := uint32(12234)

	for payload := range stream {
		switch payload.GetSource() {
		case warden.ProcessPayload_stdout:
			stdout += payload.GetData()

		case warden.ProcessPayload_stderr:
			stderr += payload.GetData()
		}

		exitStatus = payload.GetExitStatus()
	}

	return stdout, stderr, exitStatus
}

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

	Describe("a started job", func() {
		It("continues to stream", func(done Done) {
			processID, runStream, err := client.Run(handle, "while true; do echo hi; sleep 0.5; done")
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			Eventually(runStream).Should(BeClosed())

			stream, err := client.Attach(handle, processID)
			Expect(err).ToNot(HaveOccurred())

			Expect((<-stream).GetData()).To(ContainSubstring("hi\n"))

			close(done)
		}, 10.0)

		It("does not have its job ID repeated", func() {
			processID1, _, err := client.Run(handle, "while true; do echo hi; sleep 0.5; done")
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			processID2, _, err := client.Run(handle, "while true; do echo hi; sleep 0.5; done")
			Expect(err).ToNot(HaveOccurred())

			Expect(processID1).ToNot(Equal(processID2))
		})

		Context("that prints monotonously increasing output", func() {
			It("does not duplicate its output on reconnect", func(done Done) {
				receivedNumbers := make(chan int, 2048)

				processID, _, err := client.Run(
					handle,
					"for i in $(seq 10); do echo $i; sleep 0.5; done; echo goodbye; while true; do sleep 1; done",
				)
				Expect(err).ToNot(HaveOccurred())

				stream, err := client.Attach(handle, processID)
				Expect(err).ToNot(HaveOccurred())

				go streamNumbersTo(receivedNumbers, stream)

				time.Sleep(500 * time.Millisecond)

				restartServer()

				stream, err = client.Attach(handle, processID)
				Expect(err).ToNot(HaveOccurred())

				go streamNumbersTo(receivedNumbers, stream)

				lastNum := 0
				for num := range receivedNumbers {
					Expect(num).To(BeNumerically(">", lastNum))
					lastNum = num
				}

				close(done)
			}, 10.0)
		})
	})

	Describe("a memory limit", func() {
		It("is still enforced", func() {
			_, err := client.LimitMemory(handle, 32*1024*1024)
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			_, stream, err := client.Run(handle, "exec ruby -e '$stdout.sync = true; puts :hello; puts (\"x\" * 64 * 1024 * 1024).size; puts :goodbye; exit 42'")
			Expect(err).ToNot(HaveOccurred())

			// cgroups OOM killer seems to leave no trace of the process;
			// there's no exit status indicator, so just assert that the one
			// we tried to exit with after over-allocating is not seen

			stdout, _, exitStatus := readUntilExit(stream)
			Expect(stdout).To(Equal("hello\n"))
			Expect(exitStatus).ToNot(Equal(uint32(42)))
		})
	})

	Describe("a container's active job", func() {
		It("is still tracked", func() {
			processID, _, err := client.Run(handle, "while true; do echo hi; sleep 0.5; done")
			Expect(err).ToNot(HaveOccurred())

			restartServer()

			info, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetProcessIds()).To(ContainElement(uint64(processID)))
		})
	})

	Describe("a container's list of events", func() {
		It("is still reported", func() {
			_, err := client.LimitMemory(handle, 32*1024*1024)
			Expect(err).ToNot(HaveOccurred())

			// trigger 'out of memory' event
			_, _, err = client.Run(handle, "exec ruby -e '$stdout.sync = true; puts :hello; puts (\"x\" * 64 * 1024 * 1024).size; puts :goodbye; exit 42'")
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
			idA := ""
			idB := ""

			_, streamA, err := client.Run(handle, "id -u")
			Expect(err).ToNot(HaveOccurred())

			for chunk := range streamA {
				idA += chunk.GetData()
			}

			restartServer()

			createRes, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			_, streamB, err := client.Run(createRes.GetHandle(), "id -u")
			Expect(err).ToNot(HaveOccurred())

			for chunk := range streamB {
				idB += chunk.GetData()
			}

			Expect(idA).ToNot(Equal(idB))
		})
	})

	Describe("a container's grace time", func() {
		BeforeEach(func() {
			err := runner.Stop()
			Expect(err).ToNot(HaveOccurred())

			err = runner.Start("--containerGraceTime", "5")
			Expect(err).ToNot(HaveOccurred())

			res, err := client.Create()
			Expect(err).ToNot(HaveOccurred())

			handle = res.GetHandle()
		})

		It("is still enforced", func() {
			restartServer()

			listRes, err := client.List()
			Expect(err).ToNot(HaveOccurred())

			Expect(listRes.GetHandles()).To(ContainElement(handle))

			time.Sleep(6 * time.Second)

			listRes, err = client.List()
			Expect(err).ToNot(HaveOccurred())

			Expect(listRes.GetHandles()).ToNot(ContainElement(handle))
		})
	})
})

func streamNumbersTo(destination chan<- int, source <-chan *warden.ProcessPayload) {
	for out := range source {
		buf := bytes.NewBufferString(out.GetData())

		var num int

		for {
			_, err := fmt.Fscanf(buf, "%d\n", &num)
			if err == io.EOF {
				break
			}

			// got goodbye
			if err != nil {
				close(destination)
				return
			}

			destination <- num
		}
	}
}
