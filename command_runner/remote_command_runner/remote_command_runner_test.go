package remote_command_runner_test

import (
	"bytes"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/command_runner/remote_command_runner"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var _ = Describe("running commands", func() {
	var fakeRunner *fake_command_runner.FakeCommandRunner
	var remoteRunner *remote_command_runner.RemoteCommandRunner

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()

		remoteRunner = remote_command_runner.New(
			"vagrant",
			"192.168.50.4",
			2222,
			fakeRunner,
		)
	})

	It("runs them over SSH", func() {
		command := &exec.Cmd{
			Path: "ruby",
			Args: []string{"-e", "p :hi"},
			Env: []string{"A=B"},
			Stdin: bytes.NewBufferString("hello\n"),
		}


		err := remoteRunner.Run(command)
		Expect(err).ToNot(HaveOccured())

		Expect(fakeRunner).To(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "ssh",
				Args: []string{
					"-l", "vagrant", "-p", "2222", "192.168.50.4",
					"A=B ruby '-e' 'p :hi'",
				},
				Env: []string{},
				Stdin: "hello\n",
			},
		))
	})

	It("starts them over SSH", func() {
		command := &exec.Cmd{
			Path: "ruby",
			Args: []string{"-e", "p :hi"},
			Env: []string{"A=B"},
			Stdin: bytes.NewBufferString("hello\n"),
		}

		err := remoteRunner.Start(command)
		Expect(err).ToNot(HaveOccured())

		Expect(fakeRunner).To(HaveStartedExecuting(
			fake_command_runner.CommandSpec{
				Path: "ssh",
				Args: []string{
					"-l", "vagrant", "-p", "2222", "192.168.50.4",
					"A=B ruby '-e' 'p :hi'",
				},
				Env: []string{},
				Stdin: "hello\n",
			},
		))
	})
})
