package linux_backend_test

import (
	"errors"

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
				}, func() error {
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
				}, func() error {
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
