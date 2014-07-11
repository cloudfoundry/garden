package server_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/cloudfoundry-incubator/garden/warden/fakes"
)

var _ = Describe("The Warden server", func() {
	Context("when passed a socket", func() {
		It("listens on the given socket path and chmods it to 0777", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			wardenServer := server.New("unix", socketPath, 0, new(fakes.FakeBackend))

			err = wardenServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())

			stat, err := os.Stat(socketPath)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(int(stat.Mode() & 0777)).Should(Equal(0777))
		})

		It("deletes the socket file if it is already there", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			socket, err := os.Create(socketPath)
			Ω(err).ShouldNot(HaveOccurred())
			socket.WriteString("oops")
			socket.Close()

			wardenServer := server.New("unix", socketPath, 0, new(fakes.FakeBackend))

			err = wardenServer.Start()
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when passed a tcp addr", func() {
		It("listens on the given addr", func() {
			wardenServer := server.New("tcp", ":60123", 0, new(fakes.FakeBackend))

			err := wardenServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(ErrorDialing("tcp", ":60123")).ShouldNot(HaveOccurred())
		})
	})

	It("starts the backend", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Ω(err).ShouldNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		fakeBackend := new(fakes.FakeBackend)

		wardenServer := server.New("unix", socketPath, 0, fakeBackend)

		err = wardenServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(fakeBackend.StartCallCount()).Should(Equal(1))
	})

	It("destroys containers that have been idle for their grace time", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Ω(err).ShouldNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		fakeBackend := new(fakes.FakeBackend)

		doomedContainer := new(fakes.FakeContainer)

		fakeBackend.ContainersReturns([]warden.Container{doomedContainer}, nil)
		fakeBackend.GraceTimeReturns(100 * time.Millisecond)

		wardenServer := server.New("unix", socketPath, 0, fakeBackend)

		before := time.Now()

		err = wardenServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(fakeBackend.DestroyCallCount()).Should(Equal(0))
		Eventually(fakeBackend.DestroyCallCount).Should(Equal(1))

		Ω(time.Since(before)).Should(BeNumerically(">", 100*time.Millisecond))
	})

	Context("when starting the backend fails", func() {
		disaster := errors.New("oh no!")

		It("fails to start", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			fakeBackend := new(fakes.FakeBackend)
			fakeBackend.StartReturns(disaster)

			wardenServer := server.New("unix", socketPath, 0, fakeBackend)

			err = wardenServer.Start()
			Ω(err).Should(Equal(disaster))
		})
	})

	Context("when listening on the socket fails", func() {
		It("fails to start", func() {
			tmpfile, err := ioutil.TempFile(os.TempDir(), "warden-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			wardenServer := server.New(
				"unix",
				// weird scenario: /foo/X/warden.sock with X being a file
				path.Join(tmpfile.Name(), "warden.sock"),
				0,
				new(fakes.FakeBackend),
			)

			err = wardenServer.Start()
			Ω(err).Should(HaveOccurred())
		})
	})

	Describe("shutting down", func() {
		var socketPath string

		var serverBackend warden.Backend
		var fakeBackend *fakes.FakeBackend

		var wardenServer *server.WardenServer
		var wardenClient warden.Client

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath = path.Join(tmpdir, "warden.sock")
			fakeBackend = new(fakes.FakeBackend)

			serverBackend = fakeBackend

			wardenClient = client.New(connection.New("unix", socketPath))
		})

		JustBeforeEach(func() {
			wardenServer = server.New("unix", socketPath, 0, serverBackend)

			err := wardenServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())
		})

		It("stops accepting new connections", func() {
			go wardenServer.Stop()

			Eventually(ErrorDialing("unix", socketPath)).Should(HaveOccurred())
		})

		It("stops the backend", func() {
			wardenServer.Stop()

			Ω(fakeBackend.StopCallCount()).Should(Equal(1))
		})

		Context("when a Create request is in-flight", func() {
			var creating chan struct{}
			var finishCreating chan struct{}

			BeforeEach(func() {
				creating = make(chan struct{})
				finishCreating = make(chan struct{})

				fakeBackend.CreateStub = func(warden.ContainerSpec) (warden.Container, error) {
					close(creating)
					<-finishCreating
					return new(fakes.FakeContainer), nil
				}
			})

			It("waits for it to complete and stops accepting requests", func() {
				created := make(chan warden.Container, 1)

				go func() {
					defer GinkgoRecover()

					container, err := wardenClient.Create(warden.ContainerSpec{})
					Ω(err).ShouldNot(HaveOccurred())

					created <- container
				}()

				Eventually(creating).Should(BeClosed())

				stopExited := make(chan struct{})
				go func() {
					wardenServer.Stop()
					close(stopExited)
				}()

				Consistently(stopExited).ShouldNot(BeClosed())

				close(finishCreating)

				Eventually(stopExited).Should(BeClosed())
				Eventually(created).Should(Receive())

				err := wardenClient.Ping()
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when a Run request is in-flight", func() {
			It("does not wait for the request to complete", func(done Done) {
				fakeContainer := new(fakes.FakeContainer)

				fakeContainer.RunStub = func(spec warden.ProcessSpec, io warden.ProcessIO) (warden.Process, error) {
					process := new(fakes.FakeProcess)

					process.WaitStub = func() (int, error) {
						time.Sleep(time.Minute)
						return 0, nil
					}

					go func() {
						defer GinkgoRecover()

						_, err := io.Stdout.Write([]byte("msg 1\n"))
						Ω(err).ShouldNot(HaveOccurred())

						time.Sleep(time.Minute)

						_, err = io.Stdout.Write([]byte("msg 2\n"))
						Ω(err).ShouldNot(HaveOccurred())
					}()

					return process, nil
				}

				fakeBackend.CreateReturns(fakeContainer, nil)

				clientContainer, err := wardenClient.Create(warden.ContainerSpec{})
				Ω(err).ShouldNot(HaveOccurred())

				fakeBackend.LookupReturns(fakeContainer, nil)

				stdout := gbytes.NewBuffer()

				process, err := clientContainer.Run(warden.ProcessSpec{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				}, warden.ProcessIO{
					Stdout: stdout,
				})
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(stdout).Should(gbytes.Say("msg 1\n"))

				wardenServer.Stop()

				_, err = process.Wait()
				Ω(err).Should(HaveOccurred())

				close(done)
			})
		})
	})
})
