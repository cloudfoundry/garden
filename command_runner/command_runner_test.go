package command_runner_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/command_runner"
)

var _ = Describe("Running commands", func() {
	It("runs the command and returns nil", func() {
		runner := command_runner.New()

		cmd := exec.Command("ls")
		Expect(cmd.ProcessState).To(BeNil())

		err := runner.Run(cmd)
		Expect(err).ToNot(HaveOccured())

		Expect(cmd.ProcessState).ToNot(BeNil())
	})

	Context("when the command fails", func() {
		It("returns an error", func() {
			runner := command_runner.New()
			err := runner.Run(exec.Command("/bin/bash", "-c", "exit 1"))
			Expect(err).To(HaveOccured())
		})
	})
})
