package quota_manager_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend/quota_manager"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var _ = Describe("Linux Quota manager", func() {
	var fakeRunner *fake_command_runner.FakeCommandRunner
	var depotPath string
	var realMountPoint string
	var quotaManager *quota_manager.LinuxQuotaManager

	BeforeEach(func() {
		fakeRunner = fake_command_runner.New()

		tmpdir, err := ioutil.TempDir(os.TempDir(), "fake-depot")
		Expect(err).ToNot(HaveOccured())

		depotPath = tmpdir

		df := exec.Command("df", "-P", tmpdir)

		out, err := df.CombinedOutput()
		Expect(err).ToNot(HaveOccured())

		dfOutputWords := strings.Split(string(out), " ")
		realMountPoint = strings.Trim(dfOutputWords[len(dfOutputWords)-1], "\n")

		quotaManager, err = quota_manager.New(depotPath, "/root/path", fakeRunner)
		Expect(err).ToNot(HaveOccured())
	})

	Describe("initialization", func() {
		It("fails if the given path does not exist", func() {
			_, err := quota_manager.New("/bogus/path", "/root/path", fakeRunner)
			Expect(err).To(HaveOccured())
		})
	})

	Describe("setting quotas", func() {
		setQuotaPath := exec.Command("setquota").Path

		limits := backend.DiskLimits{
			BlockSoft: 1,
			BlockHard: 2,

			InodeSoft: 11,
			InodeHard: 12,
		}

		It("executes setquota on the container depo's mount point", func() {
			err := quotaManager.SetLimits(1234, limits)

			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: setQuotaPath,
					Args: []string{
						"-u", "1234",
						"1", "2", "11", "12",
						realMountPoint,
					},
				},
			))
		})

		Context("when bytes are given", func() {
			limits := backend.DiskLimits{
				InodeSoft: 11,
				InodeHard: 12,

				ByteSoft: 102401,
				ByteHard: 204801,
			}

			It("executes setquota with them converted to blocks", func() {
				err := quotaManager.SetLimits(1234, limits)

				Expect(err).ToNot(HaveOccured())

				Expect(fakeRunner).To(HaveExecutedSerially(
					fake_command_runner.CommandSpec{
						Path: setQuotaPath,
						Args: []string{
							"-u", "1234",
							"101", "201", "11", "12",
							realMountPoint,
						},
					},
				))
			})
		})

		Context("when setquota fails", func() {
			nastyError := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: setQuotaPath,
					}, func(*exec.Cmd) error {
						return nastyError
					},
				)
			})

			It("returns the error", func() {
				err := quotaManager.SetLimits(1234, limits)
				Expect(err).To(Equal(nastyError))
			})
		})
	})

	Describe("getting quotas limits", func() {
		It("executes repquota in the root path", func() {
			fakeRunner.WhenRunning(
				fake_command_runner.CommandSpec{
					Path: "/root/path/bin/repquota",
					Args: []string{realMountPoint, "1234"},
				}, func(cmd *exec.Cmd) error {
					cmd.Stdout.Write([]byte("1234 111 222 333 444 555 666 777\n"))

					return nil
				},
			)

			limits, err := quotaManager.GetLimits(1234)
			Expect(err).ToNot(HaveOccured())

			Expect(limits.BlockSoft).To(Equal(uint64(222)))
			Expect(limits.BlockHard).To(Equal(uint64(333)))

			Expect(limits.InodeSoft).To(Equal(uint64(666)))
			Expect(limits.InodeHard).To(Equal(uint64(777)))
		})

		Context("when repquota fails", func() {
			disaster := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/root/path/bin/repquota",
						Args: []string{realMountPoint, "1234"},
					}, func(cmd *exec.Cmd) error {
						return disaster
					},
				)
			})

			It("returns the error", func() {
				_, err := quotaManager.GetLimits(1234)
				Expect(err).To(Equal(disaster))
			})
		})

		Context("when the output of repquota is malformed", func() {
			It("returns an error", func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/root/path/bin/repquota",
						Args: []string{realMountPoint, "1234"},
					}, func(cmd *exec.Cmd) error {
						cmd.Stdout.Write([]byte("abc\n"))

						return nil
					},
				)

				_, err := quotaManager.GetLimits(1234)
				Expect(err).To(HaveOccured())
			})
		})
	})
})
