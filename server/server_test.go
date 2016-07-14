package server_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	fakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server"
)

var _ = Describe("The Garden server", func() {
	var logger *lagertest.TestLogger
	var tmpdir string
	var apiClient garden.Client

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		if tmpdir != "" {
			os.RemoveAll(tmpdir)
		}
	})

	Context("when passed a socket", func() {
		It("listens on the given socket path and chmods it to 0777", func() {
			var err error
			tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "api.sock")

			apiServer := server.New("unix", socketPath, 0, new(fakes.FakeBackend), logger)

			err = apiServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			apiClient = client.New(connection.New("unix", socketPath))
			Eventually(apiClient.Ping).Should(Succeed())

			stat, err := os.Stat(socketPath)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(int(stat.Mode() & 0777)).Should(Equal(0777))
		})

		It("deletes the socket file if it is already there", func() {
			var err error
			tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "api.sock")

			socket, err := os.Create(socketPath)
			Ω(err).ShouldNot(HaveOccurred())
			socket.WriteString("oops")
			socket.Close()

			apiServer := server.New("unix", socketPath, 0, new(fakes.FakeBackend), logger)

			err = apiServer.Start()
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Context("when passed a tcp addr", func() {
		It("listens on the given addr", func() {
			apiServer := server.New("tcp", ":60123", 0, new(fakes.FakeBackend), logger)

			err := apiServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			apiClient = client.New(connection.New("tcp", ":60123"))
			Eventually(apiClient.Ping).Should(Succeed())
		})
	})

	It("starts the backend", func() {
		var err error
		tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
		Ω(err).ShouldNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "api.sock")

		fakeBackend := new(fakes.FakeBackend)

		apiServer := server.New("unix", socketPath, 0, fakeBackend, logger)

		err = apiServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(fakeBackend.StartCallCount()).Should(Equal(1))
	})

	It("destroys containers that have been idle for their grace time", func() {
		var err error
		tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
		Ω(err).ShouldNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "api.sock")

		fakeBackend := new(fakes.FakeBackend)

		doomedContainer := new(fakes.FakeContainer)

		fakeBackend.ContainersReturns([]garden.Container{doomedContainer}, nil)
		fakeBackend.GraceTimeReturns(100 * time.Millisecond)

		apiServer := server.New("unix", socketPath, 0, fakeBackend, logger)

		before := time.Now()

		err = apiServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		Ω(fakeBackend.DestroyCallCount()).Should(Equal(0))
		Eventually(fakeBackend.DestroyCallCount).Should(Equal(1))

		Ω(time.Since(before)).Should(BeNumerically(">", 100*time.Millisecond))
	})

	Context("when starting the backend fails", func() {
		disaster := errors.New("oh no!")

		It("fails to start", func() {
			var err error
			tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "api.sock")

			fakeBackend := new(fakes.FakeBackend)
			fakeBackend.StartReturns(disaster)

			apiServer := server.New("unix", socketPath, 0, fakeBackend, logger)

			err = apiServer.Start()
			Ω(err).Should(Equal(disaster))
		})
	})

	Context("when listening on the socket fails", func() {
		It("fails to start", func() {
			tmpfile, err := ioutil.TempFile(os.TempDir(), "api-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			apiServer := server.New(
				"unix",
				// weird scenario: /foo/X/api.sock with X being a file
				path.Join(tmpfile.Name(), "api.sock"),
				0,
				new(fakes.FakeBackend),
				logger,
			)

			err = apiServer.Start()
			Ω(err).Should(HaveOccurred())
		})
	})

	Describe("shutting down", func() {
		var socketPath string

		var serverBackend garden.Backend
		var fakeBackend *fakes.FakeBackend

		var apiServer *server.GardenServer
		var apiClient garden.Client

		BeforeEach(func() {
			var err error
			tmpdir, err = ioutil.TempDir(os.TempDir(), "api-server-test")
			Ω(err).ShouldNot(HaveOccurred())

			socketPath = path.Join(tmpdir, "api.sock")
			fakeBackend = new(fakes.FakeBackend)

			serverBackend = fakeBackend

			apiClient = client.New(connection.New("unix", socketPath))
		})

		JustBeforeEach(func() {
			apiServer = server.New("unix", socketPath, 0, serverBackend, logger)

			err := apiServer.Start()
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(apiClient.Ping).Should(Succeed())
		})

		It("stops accepting new connections", func() {
			apiServer.Stop()

			Eventually(apiClient.Ping).ShouldNot(Succeed())
		})

		It("stops the backend", func() {
			apiServer.Stop()

			Ω(fakeBackend.StopCallCount()).Should(Equal(1))
		})

		Context("when a Create request is in-flight", func() {
			var creating chan struct{}
			var finishCreating chan struct{}

			BeforeEach(func() {
				creating = make(chan struct{})
				finishCreating = make(chan struct{})

				fakeBackend.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
					close(creating)
					<-finishCreating
					return new(fakes.FakeContainer), nil
				}
			})

			It("waits for it to complete and stops accepting requests", func() {
				created := make(chan garden.Container, 1)

				go func() {
					defer GinkgoRecover()

					container, err := apiClient.Create(garden.ContainerSpec{})
					Ω(err).ShouldNot(HaveOccurred())

					created <- container
				}()

				Eventually(creating).Should(BeClosed())

				stopExited := make(chan struct{})
				go func() {
					apiServer.Stop()
					close(stopExited)
				}()

				Consistently(stopExited).ShouldNot(BeClosed())

				close(finishCreating)

				Eventually(stopExited).Should(BeClosed())
				Eventually(created).Should(Receive())

				err := apiClient.Ping()
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when a Run request is in-flight", func() {
			It("does not wait for the request to complete", func(done Done) {
				fakeContainer := new(fakes.FakeContainer)

				fakeContainer.RunStub = func(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
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

				clientContainer, err := apiClient.Create(garden.ContainerSpec{})
				Ω(err).ShouldNot(HaveOccurred())

				fakeBackend.LookupReturns(fakeContainer, nil)

				stdout := gbytes.NewBuffer()

				process, err := clientContainer.Run(garden.ProcessSpec{
					Path: "some-path",
					Args: []string{"arg1", "arg2"},
				}, garden.ProcessIO{
					Stdout: stdout,
				})
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(stdout).Should(gbytes.Say("msg 1\n"))

				apiServer.Stop()

				_, err = process.Wait()
				Ω(err).Should(HaveOccurred())

				close(done)
			})
		})
	})
})
