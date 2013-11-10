package linux_container_pool

import (
	"os/exec"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/backend/linux_backend"
	"github.com/vito/garden/command_runner"
)

type LinuxContainerPool struct {
	rootPath   string
	depotPath  string
	rootFSPath string

	runner command_runner.CommandRunner

	nextContainer int64

	sync.RWMutex
}

func New(rootPath, depotPath, rootFSPath string, runner command_runner.CommandRunner) *LinuxContainerPool {
	return &LinuxContainerPool{
		rootPath:   rootPath,
		depotPath:  depotPath,
		rootFSPath: rootFSPath,

		runner: runner,

		nextContainer: time.Now().UnixNano(),
	}
}

func (p *LinuxContainerPool) Setup() error {
	setup := exec.Command(path.Join(p.rootPath, "setup.sh"))

	setup.Env = []string{
		"POOL_NETWORK=10.254.0.0/24",
		"ALLOW_NETWORKS=",
		"DENY_NETWORKS=",
		"CONTAINER_ROOTFS_PATH=" + p.rootFSPath,
		"CONTAINER_DEPOT_PATH=" + p.depotPath,
		"CONTAINER_DEPOT_MOUNT_POINT_PATH=/",
		"DISK_QUOTA_ENABLED=true",

		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	err := p.runner.Run(setup)
	if err != nil {
		return err
	}

	return nil
}

func (p *LinuxContainerPool) Create(spec backend.ContainerSpec) (backend.Container, error) {
	p.Lock()

	id := p.generateContainerID()

	p.Unlock()

	containerPath := path.Join(p.depotPath, id)

	container := linux_backend.NewLinuxContainer(id, containerPath, spec, p.runner)

	create := exec.Command(
		path.Join(p.rootPath, "create.sh"),
		containerPath,
	)

	create.Env = []string{
		"id=" + container.ID(),
		"rootfs_path=" + p.rootFSPath,

		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	err := p.runner.Run(create)
	if err != nil {
		return nil, err
	}

	return container, nil
}

func (p *LinuxContainerPool) Destroy(container backend.Container) error {
	destroy := exec.Command(
		path.Join(p.rootPath, "destroy.sh"),
		path.Join(p.depotPath, container.ID()),
	)

	destroy.Env = []string{
		"id=" + container.ID(),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}

	err := p.runner.Run(destroy)
	if err != nil {
		return err
	}

	return nil
}

func (p *LinuxContainerPool) generateContainerID() string {
	p.nextContainer++

	containerID := []byte{}

	var i uint
	for i = 0; i < 11; i++ {
		containerID = strconv.AppendInt(
			containerID,
			(p.nextContainer>>(55-(i+1)*5))&31,
			32,
		)
	}

	return string(containerID)
}
