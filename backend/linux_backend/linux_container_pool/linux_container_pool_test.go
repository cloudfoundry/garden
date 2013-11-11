package linux_container_pool_test

import (
	"errors"
	"net"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend/fake_network_pool"
	"github.com/vito/garden/backend/linux_backend/fake_uid_pool"
	"github.com/vito/garden/backend/linux_backend/linux_container_pool"
	"github.com/vito/garden/command_runner/fake_command_runner"
	. "github.com/vito/garden/command_runner/fake_command_runner/matchers"
)

var _ = Describe("Linux Container pool", func() {
	var fakeRunner *fake_command_runner.FakeCommandRunner
	var fakeUIDPool *fake_uid_pool.FakeUIDPool
	var fakeNetworkPool *fake_network_pool.FakeNetworkPool
	var pool *linux_container_pool.LinuxContainerPool

	BeforeEach(func() {
		fakeUIDPool = fake_uid_pool.New(10000)
		fakeNetworkPool = fake_network_pool.New(net.ParseIP("1.2.3.0"))
		fakeRunner = fake_command_runner.New()

		pool = linux_container_pool.New(
			"/root/path",
			"/depot/path",
			"/rootfs/path",
			fakeUIDPool,
			fakeNetworkPool,
			fakeRunner,
		)
	})

	var _ = Describe("setup", func() {
		It("executes setup.sh with the correct environment", func() {
			err := pool.Setup()
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/root/path/setup.sh",
					Env: []string{
						"POOL_NETWORK=10.254.0.0/24",
						"ALLOW_NETWORKS=",
						"DENY_NETWORKS=",
						"CONTAINER_ROOTFS_PATH=/rootfs/path",
						"CONTAINER_DEPOT_PATH=/depot/path",
						"CONTAINER_DEPOT_MOUNT_POINT_PATH=/",
						"DISK_QUOTA_ENABLED=true",

						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					},
				},
			))
		})

		Context("when setup.sh fails", func() {
			nastyError := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/root/path/setup.sh",
					}, func(*exec.Cmd) error {
						return nastyError
					},
				)
			})

			It("returns the error", func() {
				err := pool.Setup()
				Expect(err).To(Equal(nastyError))
			})
		})
	})

	var _ = Describe("creating", func() {
		It("returns containers with unique IDs", func() {
			container1, err := pool.Create(backend.ContainerSpec{})
			Expect(err).ToNot(HaveOccured())

			container2, err := pool.Create(backend.ContainerSpec{})
			Expect(err).ToNot(HaveOccured())

			Expect(container1.ID()).ToNot(Equal(container2.ID()))
		})

		It("executes create.sh with the correct args and environment", func() {
			container, err := pool.Create(backend.ContainerSpec{})
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/root/path/create.sh",
					Args: []string{"/depot/path/" + container.ID()},
					Env: []string{
						"id=" + container.ID(),
						"rootfs_path=/rootfs/path",
						"user_uid=10000",
						"network_host_ip=1.2.3.1",
						"network_container_ip=1.2.3.2",
						"network_netmask=255.255.255.252",

						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					},
				},
			))
		})

		Context("when acquiring a UID fails", func() {
			nastyError := errors.New("oh no!")

			JustBeforeEach(func() {
				fakeUIDPool.AcquireError = nastyError
			})

			It("returns the error", func() {
				_, err := pool.Create(backend.ContainerSpec{})
				Expect(err).To(Equal(nastyError))
			})
		})

		Context("when acquiring a network fails", func() {
			nastyError := errors.New("oh no!")

			JustBeforeEach(func() {
				fakeNetworkPool.AcquireError = nastyError
			})

			It("returns the error and releases the uid", func() {
				_, err := pool.Create(backend.ContainerSpec{})
				Expect(err).To(Equal(nastyError))

				Expect(fakeUIDPool.Released).To(ContainElement(uint32(10000)))
			})
		})

		Context("when executing create.sh fails", func() {
			nastyError := errors.New("oh no!")

			BeforeEach(func() {
				fakeRunner.WhenRunning(
					fake_command_runner.CommandSpec{
						Path: "/root/path/create.sh",
					}, func(*exec.Cmd) error {
						return nastyError
					},
				)
			})

			It("returns the error and releases the uid and network", func() {
				_, err := pool.Create(backend.ContainerSpec{})
				Expect(err).To(Equal(nastyError))

				Expect(fakeUIDPool.Released).To(ContainElement(uint32(10000)))
				Expect(fakeNetworkPool.Released).To(ContainElement("1.2.3.0/30"))
			})
		})
	})

	var _ = Describe("destroying", func() {
		It("executes destroy.sh with the correct args and environment", func() {
			container, err := pool.Create(backend.ContainerSpec{})
			Expect(err).ToNot(HaveOccured())

			err = pool.Destroy(container)
			Expect(err).ToNot(HaveOccured())

			Expect(fakeRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "/root/path/destroy.sh",
					Args: []string{"/depot/path/" + container.ID()},
					Env: []string{
						"id=" + container.ID(),
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					},
				},
			))
		})
	})

})
