package cgroups_manager_test

import (
	"io/ioutil"
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend/linux_backend/cgroups_manager"
)

var _ = Describe("Container cgroups", func() {
	var cgroupsPath string
	var cgroupsManager *cgroups_manager.ContainerCgroupsManager

	BeforeEach(func() {
		tmpdir, err := ioutil.TempDir(os.TempDir(), "some-cgroups")
		Expect(err).ToNot(HaveOccured())

		cgroupsPath = tmpdir
		cgroupsManager = cgroups_manager.New(cgroupsPath, "some-container-id")
	})

	Describe("setting and getting", func() {
		It("writes the value to the name under the subsytem", func() {
			containerMemoryCgroupsPath := path.Join(cgroupsPath, "memory", "instance-some-container-id")
			err := os.MkdirAll(containerMemoryCgroupsPath, 0755)
			Expect(err).ToNot(HaveOccured())

			err = cgroupsManager.Set("memory", "memory.limit_in_bytes", "42")
			Expect(err).ToNot(HaveOccured())

			value, err := ioutil.ReadFile(path.Join(containerMemoryCgroupsPath, "memory.limit_in_bytes"))
			Expect(err).ToNot(HaveOccured())
			Expect(string(value)).To(Equal("42"))
		})

		Context("when the cgroups directory does not exist", func() {
			BeforeEach(func() {
				err := os.RemoveAll(cgroupsPath)
				Expect(err).ToNot(HaveOccured())
			})

			It("returns an error", func() {
				err := cgroupsManager.Set("memory", "memory.limit_in_bytes", "42")
				Expect(err).To(HaveOccured())
			})
		})
	})

	Describe("getting", func() {
		It("reads the current value from the name under the subsystem", func() {
			containerMemoryCgroupsPath := path.Join(cgroupsPath, "memory", "instance-some-container-id")

			err := os.MkdirAll(containerMemoryCgroupsPath, 0755)
			Expect(err).ToNot(HaveOccured())

			err = ioutil.WriteFile(path.Join(containerMemoryCgroupsPath, "memory.limit_in_bytes"), []byte("123"), 0644)
			Expect(err).ToNot(HaveOccured())

			val, err := cgroupsManager.Get("memory", "memory.limit_in_bytes")
			Expect(err).ToNot(HaveOccured())
			Expect(val).To(Equal("123"))
		})
	})

	Describe("retrieving a subsystem path", func() {
		It("returns <path>/<subsytem>/instance-<container-id>", func() {
			Expect(cgroupsManager.SubsystemPath("memory")).To(Equal(
				path.Join(cgroupsPath, "memory", "instance-some-container-id"),
			))
		})
	})
})
