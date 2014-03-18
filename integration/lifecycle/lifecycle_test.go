package lifecycle_test

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/gordon"
)

var _ = Describe("Creating a container", func() {
	var handle string

	BeforeEach(func() {
		res, err := client.Create()
		Expect(err).ToNot(HaveOccurred())

		handle = res.GetHandle()
	})

	AfterEach(func() {
		_, err := client.Destroy(handle)
		Expect(err).ToNot(HaveOccurred())
	})

	It("sources /etc/seed", func() {
		_, stream, err := client.Run(
			handle,
			"test -e /tmp/ran-seed",
			gordon.ResourceLimits{},
		)
		Expect(err).ToNot(HaveOccurred())

		for chunk := range stream {
			if chunk.ExitStatus != nil {
				Expect(chunk.GetExitStatus()).To(Equal(uint32(0)))
			}
		}
	})

	Context("and sending a List request", func() {
		It("includes the created container", func() {
			res, err := client.List()
			Expect(err).ToNot(HaveOccurred())

			Expect(res.GetHandles()).To(ContainElement(handle))
		})
	})

	Context("and sending an Info request", func() {
		It("returns the container's info", func() {
			res, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(res.GetState()).To(Equal("active"))
		})
	})

	Context("and running a job", func() {
		It("sends output back in chunks until stopped", func() {
			_, stream, err := client.Run(
				handle,
				"sleep 0.5; echo hello; sleep 0.5; echo goodbye; sleep 0.5; exit 42",
				gordon.ResourceLimits{},
			)
			Expect(err).ToNot(HaveOccurred())

			Expect((<-stream).GetData()).To(Equal("hello\n"))
			Expect((<-stream).GetData()).To(Equal("goodbye\n"))
			Expect((<-stream).GetExitStatus()).To(Equal(uint32(42)))
		})

		Context("and then attaching to it", func() {
			It("sends output back in chunks until stopped", func(done Done) {
				processID, _, err := client.Run(
					handle,
					"sleep 2; echo hello; sleep 0.5; echo goodbye; sleep 0.5; exit 42",
					gordon.ResourceLimits{},
				)
				Expect(err).ToNot(HaveOccurred())

				stream, err := client.Attach(handle, processID)

				Expect((<-stream).GetData()).To(Equal("hello\n"))
				Expect((<-stream).GetData()).To(Equal("goodbye\n"))
				Expect((<-stream).GetExitStatus()).To(Equal(uint32(42)))

				close(done)
			}, 10.0)
		})

		Context("and then sending a Stop request", func() {
			It("kills the job", func(done Done) {
				_, stream, err := client.Run(
					handle,
					`exec ruby -e 'trap("TERM") { exit 42 }; while true; sleep 1; end'`,
					gordon.ResourceLimits{},
				)

				Expect(err).ToNot(HaveOccurred())

				_, err = client.Stop(handle, false, false)
				Expect(err).ToNot(HaveOccurred())

				Expect((<-stream).GetExitStatus()).To(Equal(uint32(42)))

				close(done)
			}, 10.0)
		})
	})

	Context("and copying files in", func() {
		var path string

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir("", "some-temp-dir-parent")
			Ω(err).ShouldNot(HaveOccurred())

			path = filepath.Join(tmpdir, "some-temp-dir")

			err = os.MkdirAll(path, 0755)
			Ω(err).ShouldNot(HaveOccurred())

			err = ioutil.WriteFile(filepath.Join(path, "some-temp-file"), []byte("HGJMT<"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

		})

		It("creates the files in the container", func() {
			_, err := client.CopyIn(handle, path, "/tmp/some-container-dir")
			Ω(err).ShouldNot(HaveOccurred())

			_, stream, err := client.Run(
				handle,
				`test -f /tmp/some-container-dir/some-temp-dir/some-temp-file && exit 42`,
				gordon.ResourceLimits{},
			)

			Expect((<-stream).GetExitStatus()).To(Equal(uint32(42)))
		})

		Context("with a strailing slash on the destination", func() {
			It("does what rsync does (syncs contents)", func() {
				_, err := client.CopyIn(handle, path+"/", "/tmp/some-container-dir/")
				Ω(err).ShouldNot(HaveOccurred())

				_, stream, err := client.Run(
					handle,
					`test -f /tmp/some-container-dir/some-temp-file && exit 42`,
					gordon.ResourceLimits{},
				)

				Expect((<-stream).GetExitStatus()).To(Equal(uint32(42)))
			})
		})

		Context("and then copying them out", func() {
			It("copies the files to the host", func() {
				_, stream, err := client.Run(
					handle,
					`mkdir -p some-container-dir; touch some-container-dir/some-file;`,
					gordon.ResourceLimits{},
				)

				Expect((<-stream).GetExitStatus()).To(Equal(uint32(0)))

				tmpdir, err := ioutil.TempDir("", "copy-out-temp-dir-parent")
				Ω(err).ShouldNot(HaveOccurred())

				user, err := user.Current()
				Ω(err).ShouldNot(HaveOccurred())

				_, err = client.CopyOut(handle, "some-container-dir", tmpdir, user.Username)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = os.Stat(filepath.Join(tmpdir, "some-container-dir", "some-file"))
				Ω(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Context("and sending a Stop request", func() {
		It("changes the container's state to 'stopped'", func() {
			_, err := client.Stop(handle, false, false)
			Expect(err).ToNot(HaveOccurred())

			info, err := client.Info(handle)
			Expect(err).ToNot(HaveOccurred())

			Expect(info.GetState()).To(Equal("stopped"))
		})
	})
})
