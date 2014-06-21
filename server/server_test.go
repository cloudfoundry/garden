package server_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/cloudfoundry-incubator/garden/warden/fake_backend"
)

var _ = Describe("The Warden server", func() {
	Context("when passed a socket", func() {
		It("listens on the given socket path and chmods it to 0777", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			wardenServer := server.New("unix", socketPath, 0, fake_backend.New())

			err = wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())

			Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())

			stat, err := os.Stat(socketPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(int(stat.Mode() & 0777)).To(Equal(0777))
		})

		It("deletes the socket file if it is already there", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			socket, err := os.Create(socketPath)
			Expect(err).ToNot(HaveOccurred())
			socket.WriteString("oops")
			socket.Close()

			wardenServer := server.New("unix", socketPath, 0, fake_backend.New())

			err = wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when passed a tcp addr", func() {
		It("listens on the given addr", func() {
			wardenServer := server.New("tcp", ":60123", 0, fake_backend.New())

			err := wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())

			Eventually(ErrorDialing("tcp", ":60123")).ShouldNot(HaveOccurred())
		})
	})

	It("starts the backend", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		fakeBackend := fake_backend.New()

		wardenServer := server.New("unix", socketPath, 0, fakeBackend)

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())

		Expect(fakeBackend.Started).To(BeTrue())
	})

	It("destroys containers that have been idle for their grace time", func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
		Expect(err).ToNot(HaveOccurred())

		socketPath := path.Join(tmpdir, "warden.sock")

		fakeBackend := fake_backend.New()

		_, err = fakeBackend.Create(warden.ContainerSpec{
			Handle:    "doomed",
			GraceTime: 100 * time.Millisecond,
		})
		Expect(err).ToNot(HaveOccurred())

		wardenServer := server.New("unix", socketPath, 0, fakeBackend)

		before := time.Now()

		err = wardenServer.Start()
		Expect(err).ToNot(HaveOccurred())

		_, err = fakeBackend.Lookup("doomed")
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			_, err := fakeBackend.Lookup("doomed")
			return err
		}).Should(HaveOccurred())

		Expect(time.Since(before)).To(BeNumerically(">", 100*time.Millisecond))
	})

	Context("when starting the backend fails", func() {
		disaster := errors.New("oh no!")

		It("fails to start", func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			socketPath := path.Join(tmpdir, "warden.sock")

			fakeBackend := fake_backend.New()
			fakeBackend.StartError = disaster

			wardenServer := server.New("unix", socketPath, 0, fakeBackend)

			err = wardenServer.Start()
			Expect(err).To(Equal(disaster))
		})
	})

	Context("when listening on the socket fails", func() {
		It("fails to start", func() {
			tmpfile, err := ioutil.TempFile(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			wardenServer := server.New(
				"unix",
				// weird scenario: /foo/X/warden.sock with X being a file
				path.Join(tmpfile.Name(), "warden.sock"),
				0,
				fake_backend.New(),
			)

			err = wardenServer.Start()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("shutting down", func() {
		var socketPath string

		var serverBackend warden.Backend
		var fakeBackend *fake_backend.FakeBackend

		var wardenServer *server.WardenServer
		var wardenClient warden.Client

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "warden-server-test")
			Expect(err).ToNot(HaveOccurred())

			socketPath = path.Join(tmpdir, "warden.sock")
			fakeBackend = fake_backend.New()

			serverBackend = fakeBackend

			wardenClient = client.New(connection.New("unix", socketPath))
		})

		JustBeforeEach(func() {
			wardenServer = server.New("unix", socketPath, 0, serverBackend)

			err := wardenServer.Start()
			Expect(err).ToNot(HaveOccurred())

			Eventually(ErrorDialing("unix", socketPath)).ShouldNot(HaveOccurred())
		})

		It("stops accepting new connections", func() {
			go wardenServer.Stop()

			Eventually(ErrorDialing("unix", socketPath)).Should(HaveOccurred())
		})

		It("stops the backend", func() {
			wardenServer.Stop()

			Expect(fakeBackend.Stopped).To(BeTrue())
		})

		Context("when a Create request is in-flight", func() {
			var creating chan struct{}

			BeforeEach(func() {
				creating = make(chan struct{})

				fakeBackend.WhenCreating = func() {
					close(creating)
					time.Sleep(500 * time.Millisecond)
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

				before := time.Now()

				wardenServer.Stop()

				Expect(time.Since(before)).To(BeNumerically("~", 500*time.Millisecond, 100*time.Millisecond))

				Eventually(created).Should(Receive())

				err := wardenClient.Ping()
				Ω(err).Should(HaveOccurred())
			})
		})
	})
})
