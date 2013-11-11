package job_tracker_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend/linux_backend/job_tracker"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var fakeRunner *fake_command_runner.FakeCommandRunner
var jobTracker *job_tracker.JobTracker

var _ = Describe("Spawning jobs", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		jobTracker = job_tracker.New(fakeRunner)

		// really run the commands
		fakeRunner.WhenRunning(
			fake_command_runner.CommandSpec{},
			func(cmd *exec.Cmd) error {
				return cmd.Run()
			},
		)
	})

	It("runs the command asynchronously", func() {
		jobTracker.Spawn(exec.Command("/bin/bash", "-c", "echo hi"))

		Eventually(fakeRunner).Should(HaveExecutedSerially(
			fake_command_runner.CommandSpec{
				Path: "/bin/bash",
			},
		))
	})

	It("returns a unique job ID", func() {
		jobID1 := jobTracker.Spawn(exec.Command("/bin/bash", "-c", "echo hi"))
		jobID2 := jobTracker.Spawn(exec.Command("/bin/bash", "-c", "echo hi"))
		Expect(jobID1).ToNot(Equal(jobID2))
	})
})

var _ = Describe("Linking to jobs", func() {
	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()
		jobTracker = job_tracker.New(fakeRunner)

		// really run the commands
		fakeRunner.WhenRunning(
			fake_command_runner.CommandSpec{},
			func(cmd *exec.Cmd) error {
				return cmd.Run()
			},
		)
	})

	It("returns their stdout, stderr, and exit status", func() {
		jobID := jobTracker.Spawn(exec.Command(
			"/bin/bash", "-c", "echo hi out; echo hi err 1>&2; exit 42",
		))

		exitStatus, stdout, stderr, err := jobTracker.Link(jobID)
		Expect(err).ToNot(HaveOccured())
		Expect(exitStatus).To(Equal(uint32(42)))
		Expect(stdout).To(Equal([]byte("hi out\n")))
		Expect(stderr).To(Equal([]byte("hi err\n")))
	})

	Context("when more than one link is made", func() {
		It("returns to both", func(done Done) {
			jobID := jobTracker.Spawn(exec.Command(
				"/bin/bash",
				"-c",

				// sleep 0.1s to give both threads time to link
				"sleep 0.1; echo hi out; echo hi err 1>&2; exit 42",
			))

			finishedLink := make(chan bool)

			go func() {
				exitStatus, stdout, stderr, err := jobTracker.Link(jobID)
				Expect(err).ToNot(HaveOccured())
				Expect(exitStatus).To(Equal(uint32(42)))
				Expect(stdout).To(Equal([]byte("hi out\n")))
				Expect(stderr).To(Equal([]byte("hi err\n")))

				finishedLink <- true
			}()

			go func() {
				exitStatus, stdout, stderr, err := jobTracker.Link(jobID)
				Expect(err).ToNot(HaveOccured())
				Expect(exitStatus).To(Equal(uint32(42)))
				Expect(stdout).To(Equal([]byte("hi out\n")))
				Expect(stderr).To(Equal([]byte("hi err\n")))

				finishedLink <- true
			}()

			<-finishedLink
			<-finishedLink

			close(done)
		}, 10.0)
	})
})
