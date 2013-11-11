package linux_backend_test

import (
	"errors"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var fakeRunner *fake_command_runner.FakeCommandRunner
var container *linux_backend.LinuxContainer

var _ = Describe("Starting", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	It("executes the container's start.sh with the correct environment", func() {
		err := container.Start()
		Expect(err).ToNot(HaveOccured())

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "/depot/some-id/start.sh",
				Env: []string{
					"id=some-id",
					"container_iface_mtu=1500",
					"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				},
			},
		))
	})

	Context("when start.sh fails", func() {
		nastyError := errors.New("oh no!")

		BeforeEach(func() {
			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/start.sh",
				}, func(*exec.Cmd) error {
					return nastyError
				},
			)
		})

		It("returns the error", func() {
			err := container.Start()
			Expect(err).To(Equal(nastyError))
		})
	})
})

var _ = Describe("Stopping", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	It("executes the container's stop.sh", func() {
		err := container.Stop(false)
		Expect(err).ToNot(HaveOccured())

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "/depot/some-id/stop.sh",
			},
		))
	})

	Context("when kill is true", func() {
		It("executes stop.sh with -w 0", func() {
			err := container.Stop(true)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/stop.sh",
					Args: []string{"-w", "0"},
				},
			))
		})
	})

	Context("when stop.sh fails", func() {
		nastyError := errors.New("oh no!")

		BeforeEach(func() {
			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/stop.sh",
				}, func(*exec.Cmd) error {
					return nastyError
				},
			)
		})

		It("returns the error", func() {
			err := container.Stop(false)
			Expect(err).To(Equal(nastyError))
		})
	})
})

var _ = Describe("Copying in", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	It("executes rsync from src into dst via wsh --rsh", func() {
		err := container.CopyIn("/src", "/dst")
		Expect(err).ToNot(HaveOccured())

		rsyncPath, err := exec.LookPath("rsync")
		if err != nil {
			rsyncPath = "rsync"
		}

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: rsyncPath,
				Args: []string{
					"-e",
					"/depot/some-id/bin/wsh --socket /depot/some-id/run/wshd.sock --rsh",
					"-r",
					"-p",
					"--links",
					"/src",
					"vcap@container:/dst",
				},
			},
		))
	})

	Context("when rsync fails", func() {
		nastyError := errors.New("oh no!")

		BeforeEach(func() {
			rsyncPath, err := exec.LookPath("rsync")
			if err != nil {
				rsyncPath = "rsync"
			}

			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: rsyncPath,
				}, func(*exec.Cmd) error {
					return nastyError
				},
			)
		})

		It("returns the error", func() {
			err := container.CopyIn("/src", "/dst")
			Expect(err).To(Equal(nastyError))
		})
	})
})

var _ = Describe("Copying out", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	It("rsyncs from vcap@container:/src to /dst", func() {
		err := container.CopyOut("/src", "/dst", "")
		Expect(err).ToNot(HaveOccured())

		rsyncPath, err := exec.LookPath("rsync")
		if err != nil {
			rsyncPath = "rsync"
		}

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: rsyncPath,
				Args: []string{
					"-e",
					"/depot/some-id/bin/wsh --socket /depot/some-id/run/wshd.sock --rsh",
					"-r",
					"-p",
					"--links",
					"vcap@container:/src",
					"/dst",
				},
			},
		))
	})

	Context("when an owner is given", func() {
		It("chowns the files after rsyncing", func() {
			err := container.CopyOut("/src", "/dst", "some-user")
			Expect(err).ToNot(HaveOccured())

			rsyncPath, err := exec.LookPath("rsync")
			if err != nil {
				rsyncPath = "rsync"
			}

			chownPath, err := exec.LookPath("chown")
			if err != nil {
				chownPath = "chown"
			}

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: rsyncPath,
				},
				fake_command_runner.CommandSpec{
					Path: chownPath,
					Args: []string{"-R", "some-user", "/dst"},
				},
			))
		})
	})

	Context("when rsync fails", func() {
		nastyError := errors.New("oh no!")

		BeforeEach(func() {
			rsyncPath, err := exec.LookPath("rsync")
			if err != nil {
				rsyncPath = "rsync"
			}

			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: rsyncPath,
				}, func(*exec.Cmd) error {
					return nastyError
				},
			)
		})

		It("returns the error", func() {
			err := container.CopyOut("/src", "/dst", "")
			Expect(err).To(Equal(nastyError))
		})
	})

	Context("when chowning fails", func() {
		nastyError := errors.New("oh no!")

		BeforeEach(func() {
			chownPath, err := exec.LookPath("chown")
			if err != nil {
				chownPath = "chown"
			}

			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: chownPath,
				}, func(*exec.Cmd) error {
					return nastyError
				},
			)
		})

		It("returns the error", func() {
			err := container.CopyOut("/src", "/dst", "some-user")
			Expect(err).To(Equal(nastyError))
		})
	})
})

var _ = Describe("Spawning", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	It("runs the /bin/bash via wsh with the given script as the input", func() {
		_, err := container.Spawn(backend.JobSpec{
			Script: "/some/script",
		})

		Expect(err).ToNot(HaveOccured())

		Eventually(fakeRunner).Should(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "/depot/some-id/bin/wsh",
				Args: []string{
					"--socket", "/depot/some-id/run/wshd.sock",
					"--user", "vcap",
					"/bin/bash",
				},
				Stdin: "/some/script",
			},
		))
	})

	It("returns a unique job ID", func() {
		jobID1, err := container.Spawn(backend.JobSpec{
			Script: "/some/script",
		})
		Expect(err).ToNot(HaveOccured())

		jobID2, err := container.Spawn(backend.JobSpec{
			Script: "/some/script",
		})
		Expect(err).ToNot(HaveOccured())

		Expect(jobID1).ToNot(Equal(jobID2))
	})

	Context("with 'privileged' true", func() {
		It("runs with --user root", func() {
			_, err := container.Spawn(backend.JobSpec{
				Script:     "/some/script",
				Privileged: true,
			})

			Expect(err).ToNot(HaveOccured())

			Eventually(fakeRunner).Should(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/wsh",
					Args: []string{
						"--socket", "/depot/some-id/run/wshd.sock",
						"--user", "root",
						"/bin/bash",
					},
					Stdin: "/some/script",
				},
			))
		})
	})
})

var _ = Describe("Linking", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		container = linux_backend.NewLinuxContainer("some-id", "/depot/some-id", backend.ContainerSpec{}, fakeRunner)
	})

	Context("to a started job", func() {
		BeforeEach(func() {
			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path:  "/depot/some-id/bin/wsh",
					Stdin: "/some/script",
				}, func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("hi out\n"))
					cmd.Stderr.Write([]byte("hi err\n"))

					dummyCmd := exec.Command("/bin/bash", "-c", "exit 42")
					dummyCmd.Run()

					cmd.ProcessState = dummyCmd.ProcessState

					return nil
				},
			)
		})

		It("returns the exit status, stdout, and stderr", func() {
			jobID, err := container.Spawn(backend.JobSpec{
				Script: "/some/script",
			})
			Expect(err).ToNot(HaveOccured())

			jobResult, err := container.Link(jobID)
			Expect(err).ToNot(HaveOccured())
			Expect(jobResult.ExitStatus).To(Equal(uint32(42)))
			Expect(jobResult.Stdout).To(Equal([]byte("hi out\n")))
			Expect(jobResult.Stderr).To(Equal([]byte("hi err\n")))
		})
	})

	Context("to a job that has already completed", func() {
		It("returns an error", func() {
			jobID, err := container.Spawn(backend.JobSpec{
				Script: "/some/script",
			})
			Expect(err).ToNot(HaveOccured())

			time.Sleep(100 * time.Millisecond)

			_, err = container.Link(jobID)
			Expect(err).To(HaveOccured())
		})
	})

	Context("to an unknown job", func() {
		It("returns an error", func() {
			_, err := container.Link(42)
			Expect(err).To(HaveOccured())
		})
	})
})
