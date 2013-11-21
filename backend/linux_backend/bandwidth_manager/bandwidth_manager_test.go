package bandwidth_manager_test

import (
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend/bandwidth_manager"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var _ = Describe("setting rate limits", func() {
	var fakeRunner *fake_command_runner.FakeCommandRunner
	var bandwidthManager *bandwidth_manager.ContainerBandwidthManager

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		bandwidthManager = bandwidth_manager.New("/depot/some-id", fakeRunner)
	})

	It("executes net_rate.sh with the appropriate environment", func() {
		limits := backend.BandwidthLimits{
			RateInBytesPerSecond:      128,
			BurstRateInBytesPerSecond: 256,
		}

		err := bandwidthManager.SetLimits(limits)
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
			err := bandwidthManager.SetLimits(backend.BandwidthLimits{
				RateInBytesPerSecond:      128,
				BurstRateInBytesPerSecond: 256,
			})
			Expect(err).To(Equal(nastyError))
		})
	})
})
