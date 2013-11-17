package linux_backend_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend"
	"github.com/vito/garden/backend/linux_backend/cgroups_manager/fake_cgroups_manager"
	"github.com/vito/garden/backend/linux_backend/port_pool"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var fakeCgroups *fake_cgroups_manager.FakeCgroupsManager
var fakeRunner *fake_command_runner.FakeCommandRunner
var container *linux_backend.LinuxContainer
var portPool *port_pool.PortPool

var _ = Describe("Linux containers", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		fakeCgroups = fake_cgroups_manager.New("/cgroups", "some-id")
		portPool = port_pool.New(1000, 10)
		container = linux_backend.NewLinuxContainer(
			"some-id",
			"/depot/some-id",
			backend.ContainerSpec{},
			portPool,
			fakeRunner,
			fakeCgroups,
		)
	})

	Describe("Starting", func() {
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

	Describe("Stopping", func() {
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

		Context("when the container has an oom notifier running", func() {
			BeforeEach(func() {
				_, err := container.LimitMemory(backend.MemoryLimits{
					LimitInBytes: 42,
				})

				Expect(err).ToNot(HaveOccured())
			})

			It("stops it", func() {
				err := container.Stop(false)
				Expect(err).ToNot(HaveOccured())

				Expect(fakeRunner).To(HaveKilled(fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/oom",
				}))
			})
		})
	})

	Describe("Copying in", func() {
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

	Describe("Copying out", func() {
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

	Describe("Jobs", func() {
		var containerPath string

		inContainer := func(x string) string {
			return path.Join(containerPath, x)
		}

		setupSuccessfulSpawn := func() {
			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: inContainer("bin/iomux-spawn"),
				},
				func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("ready\n"))
					cmd.Stdout.Write([]byte("active\n"))
					return nil
				},
			)
		}

		BeforeEach(func() {
			tmpdir, err := ioutil.TempDir(os.TempDir(), "some-container")
			Expect(err).ToNot(HaveOccured())

			containerPath = tmpdir

			container = linux_backend.NewLinuxContainer(
				"some-id",
				containerPath,
				backend.ContainerSpec{},
				portPool,
				fakeRunner,
				fakeCgroups,
			)
		})

		Describe("Spawning", func() {
			It("runs the /bin/bash via wsh with the given script as the input", func() {
				setupSuccessfulSpawn()

				jobID, err := container.Spawn(backend.JobSpec{
					Script: "/some/script",
				})

				Expect(err).ToNot(HaveOccured())

				Eventually(fakeRunner).Should(HaveStartedExecuting(
					fake_command_runner.CommandSpec{
						Path: inContainer("bin/iomux-spawn"),
						Args: []string{
							fmt.Sprintf(inContainer("jobs/%d"), jobID),
							inContainer("bin/wsh"),
							"--socket", inContainer("run/wshd.sock"),
							"--user", "vcap",
							"/bin/bash",
						},
						Stdin: "/some/script",
					},
				))
			})

			It("returns a unique job ID", func() {
				setupSuccessfulSpawn()

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
				BeforeEach(setupSuccessfulSpawn)

				It("runs with --user root", func() {
					jobID, err := container.Spawn(backend.JobSpec{
						Script:     "/some/script",
						Privileged: true,
					})

					Expect(err).ToNot(HaveOccured())

					Eventually(fakeRunner).Should(HaveStartedExecuting(
						fake_command_runner.CommandSpec{
							Path: inContainer("bin/iomux-spawn"),
							Args: []string{
								fmt.Sprintf(inContainer("jobs/%d"), jobID),
								inContainer("bin/wsh"),
								"--socket", inContainer("run/wshd.sock"),
								"--user", "root",
								"/bin/bash",
							},
							Stdin: "/some/script",
						},
					))
				})
			})

			Context("when spawning fails", func() {
				disaster := errors.New("oh no!")

				BeforeEach(func() {
					fakeRunner.WhenRunning(
						fake_command_runner.CommandSpec{
							Path: inContainer("bin/iomux-spawn"),
						}, func(*exec.Cmd) error {
							return disaster
						},
					)
				})

				It("returns the error", func() {
					_, err := container.Spawn(backend.JobSpec{
						Script:     "/some/script",
						Privileged: true,
					})

					Expect(err).To(Equal(disaster))
				})
			})
		})

		Describe("Linking", func() {
			BeforeEach(setupSuccessfulSpawn)

			Context("to a started job", func() {
				BeforeEach(func() {
					fakeRunner.WhenRunning(
						fake_command_runner.CommandSpec{
							Path: inContainer("bin/iomux-link"),
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
	})

	Describe("Limiting bandwidth", func() {
		It("executes net_rate.sh with the appropriate environment", func() {
			limits := backend.BandwidthLimits{
				RateInBytesPerSecond:      128,
				BurstRateInBytesPerSecond: 256,
			}

			newLimits, err := container.LimitBandwidth(limits)

			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/net_rate.sh",
					Env: []string{
						"BURST=256",
						fmt.Sprintf("RATE=%d", 128*8),
					},
				},
			))

			Expect(newLimits).To(Equal(limits))
		})

		Context("when net_rate.sh fails", func() {
			nastyError := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/net_rate.sh",
					}, func(*exec.Cmd) error {
						return nastyError
					},
				)
			})

			It("returns the error", func() {
				_, err := container.LimitBandwidth(backend.BandwidthLimits{
					RateInBytesPerSecond:      128,
					BurstRateInBytesPerSecond: 256,
				})
				Expect(err).To(Equal(nastyError))
			})
		})
	})

	Describe("Limiting memory", func() {
		It("starts the oom notifier", func() {
			limits := backend.MemoryLimits{
				LimitInBytes: 102400,
			}

			_, err := container.LimitMemory(limits)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveStartedExecuting(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/oom",
					Args: []string{"/cgroups/memory/instance-some-id"},
				},
			))
		})

		It("sets memory.limit_in_bytes and then memory.memsw.limit_in_bytes", func() {
			limits := backend.MemoryLimits{
				LimitInBytes: 102400,
			}

			_, err := container.LimitMemory(limits)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeCgroups.SetValues()).To(Equal(
				[]fake_cgroups_manager.SetValue{
					{
						Subsystem: "memory",
						Name:      "memory.limit_in_bytes",
						Value:     "102400",
					},
					{
						Subsystem: "memory",
						Name:      "memory.memsw.limit_in_bytes",
						Value:     "102400",
					},
					{
						Subsystem: "memory",
						Name:      "memory.limit_in_bytes",
						Value:     "102400",
					},
				},
			))
		})

		It("returns the limited memory", func() {
			limits := backend.MemoryLimits{
				LimitInBytes: 102400,
			}

			fakeCgroups.WhenGetting("memory", "memory.limit_in_bytes", func() (string, error) {
				return "18446744073709551615", nil
			})

			actualLimits, err := container.LimitMemory(limits)
			Expect(err).ToNot(HaveOccured())

			Expect(actualLimits.LimitInBytes).To(Equal(uint64(math.MaxUint64)))
		})

		Context("when the oom notifier is already running", func() {
			It("does not start another", func() {
				started := 0

				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/oom",
				}, func(*exec.Cmd) error {
					started++
					return nil
				})

				limits := backend.MemoryLimits{
					LimitInBytes: 102400,
				}

				_, err := container.LimitMemory(limits)
				Expect(err).ToNot(HaveOccured())

				_, err = container.LimitMemory(limits)
				Expect(err).ToNot(HaveOccured())

				Expect(started).To(Equal(1))
			})
		})

		Context("when the oom notifier exits 0", func() {
			BeforeEach(func() {
				fakeRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/oom",
				}, func(cmd *exec.Cmd) error {
					return nil
				})
			})

			It("stops the container", func() {
				limits := backend.MemoryLimits{
					LimitInBytes: 102400,
				}

				_, err := container.LimitMemory(limits)
				Expect(err).ToNot(HaveOccured())

				Eventually(fakeRunner).Should(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/stop.sh",
					},
				))
			})
		})

		Context("when setting memory.memsw.limit_in_bytes fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeCgroups.WhenSetting("memory", "memory.memsw.limit_in_bytes", func() error {
					return disaster
				})
			})

			It("returns the error and no limits", func() {
				limits, err := container.LimitMemory(backend.MemoryLimits{
					LimitInBytes: 102400,
				})

				Expect(err).To(Equal(disaster))
				Expect(limits).To(BeZero())
			})
		})

		Context("when setting memory.limit_in_bytes fails only the first time", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				numSet := 0

				fakeCgroups.WhenSetting("memory", "memory.limit_in_bytes", func() error {
					numSet++

					if numSet == 1 {
						return disaster
					}

					return nil
				})
			})

			It("succeeds", func() {
				fakeCgroups.WhenGetting("memory", "memory.limit_in_bytes", func() (string, error) {
					return "123", nil
				})

				limits, err := container.LimitMemory(backend.MemoryLimits{
					LimitInBytes: 102400,
				})

				Expect(err).ToNot(HaveOccured())
				Expect(limits.LimitInBytes).To(Equal(uint64(123)))
			})
		})

		Context("when setting memory.limit_in_bytes fails the second time", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				numSet := 0

				fakeCgroups.WhenSetting("memory", "memory.limit_in_bytes", func() error {
					numSet++

					if numSet == 2 {
						return disaster
					}

					return nil
				})
			})

			It("returns the error and no limits", func() {
				limits, err := container.LimitMemory(backend.MemoryLimits{
					LimitInBytes: 102400,
				})

				Expect(err).To(Equal(disaster))
				Expect(limits).To(BeZero())
			})
		})

		Context("when starting the oom notifier fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "/depot/some-id/bin/oom",
				}, func(cmd *exec.Cmd) error {
					return disaster
				})
			})

			It("returns the error and no limits", func() {
				limits, err := container.LimitMemory(backend.MemoryLimits{
					LimitInBytes: 102400,
				})

				Expect(err).To(Equal(disaster))
				Expect(limits).To(BeZero())
			})
		})
	})

	Describe("Net in", func() {
		It("executes net.sh in with HOST_PORT and CONTAINER_PORT", func() {
			hostPort, containerPort, err := container.NetIn(123, 456)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/net.sh",
					Args: []string{"in"},
					Env: []string{
						"HOST_PORT=123",
						"CONTAINER_PORT=456",
					},
				},
			))

			Expect(hostPort).To(Equal(uint32(123)))
			Expect(containerPort).To(Equal(uint32(456)))
		})

		Context("when a host port is not provided", func() {
			It("acquires one from the port pool", func() {
				hostPort, containerPort, err := container.NetIn(0, 456)
				Expect(err).ToNot(HaveOccured())

				Expect(hostPort).To(Equal(uint32(1000)))
				Expect(containerPort).To(Equal(uint32(456)))

				secondHostPort, _, err := container.NetIn(0, 456)
				Expect(err).ToNot(HaveOccured())

				Expect(secondHostPort).ToNot(Equal(hostPort))
			})
		})

		Context("when a container port is not provided", func() {
			It("defaults it to the host port", func() {
				hostPort, containerPort, err := container.NetIn(123, 0)
				Expect(err).ToNot(HaveOccured())

				Expect(fakeRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/net.sh",
						Args: []string{"in"},
						Env: []string{
							"HOST_PORT=123",
							"CONTAINER_PORT=123",
						},
					},
				))

				Expect(hostPort).To(Equal(uint32(123)))
				Expect(containerPort).To(Equal(uint32(123)))
			})

			Context("and a host port is not provided either", func() {
				It("defaults it to the same acquired port", func() {
					hostPort, containerPort, err := container.NetIn(0, 0)
					Expect(err).ToNot(HaveOccured())

					Expect(fakeRunner).To(HaveExecutedSerially(
						fake_command_runner.CommandSpec{
							Path: "/depot/some-id/net.sh",
							Args: []string{"in"},
							Env: []string{
								"HOST_PORT=1000",
								"CONTAINER_PORT=1000",
							},
						},
					))

					Expect(hostPort).To(Equal(uint32(1000)))
					Expect(containerPort).To(Equal(uint32(1000)))
				})
			})
		})

		Context("when net.sh fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/net.sh",
					}, func(*exec.Cmd) error {
						return disaster
					},
				)
			})

			It("returns the error", func() {
				_, _, err := container.NetIn(123, 456)
				Expect(err).To(Equal(disaster))
			})
		})
	})

	Describe("Net out", func() {
		It("executes net.sh out with NETWORK and PORT", func() {
			err := container.NetOut("1.2.3.4/22", 567)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/depot/some-id/net.sh",
					Args: []string{"out"},
					Env: []string{
						"NETWORK=1.2.3.4/22",
						"PORT=567",
					},
				},
			))
		})

		Context("when port 0 is given", func() {
			It("executes with PORT as an empty string", func() {
				err := container.NetOut("1.2.3.4/22", 0)
				Expect(err).ToNot(HaveOccured())

				Expect(fakeRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/net.sh",
						Args: []string{"out"},
						Env: []string{
							"NETWORK=1.2.3.4/22",
							"PORT=",
						},
					},
				))
			})
		})

		Context("when net.sh fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/depot/some-id/net.sh",
					}, func(*exec.Cmd) error {
						return disaster
					},
				)
			})

			It("returns the error", func() {
				err := container.NetOut("1.2.3.4/22", 567)
				Expect(err).To(Equal(disaster))
			})
		})
	})
})
