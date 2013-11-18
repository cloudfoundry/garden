package quota_manager

import (
	"os"
	"os/exec"
	"fmt"
	"syscall"
	"path"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/command_runner"
)

type QuotaManager interface {
	SetLimits(uid uint32, limits backend.DiskLimits) error
	GetLimits(uid uint32) (backend.DiskLimits, error)
}

type LinuxQuotaManager struct {
	mountPoint string

	rootPath string

	runner command_runner.CommandRunner
}

const QUOTA_BLOCK_SIZE = 1024

func New(containerDepotPath, rootPath string, runner command_runner.CommandRunner) (*LinuxQuotaManager, error) {
	mountPoint, err := findMountPoint(containerDepotPath)
	if err != nil {
		return nil, err
	}

	return &LinuxQuotaManager{
		rootPath: rootPath,
		runner: runner,

		mountPoint: mountPoint,
	}, nil
}

func (m *LinuxQuotaManager) SetLimits(uid uint32, limits backend.DiskLimits) error {
	if limits.ByteSoft != 0 {
		limits.BlockSoft = (limits.ByteSoft + QUOTA_BLOCK_SIZE - 1) / QUOTA_BLOCK_SIZE
	}

	if limits.ByteHard != 0 {
		limits.BlockHard = (limits.ByteHard + QUOTA_BLOCK_SIZE - 1) / QUOTA_BLOCK_SIZE
	}

	return m.runner.Run(
		exec.Command(
			"setquota",
			"-u",
			fmt.Sprintf("%d", uid),
			fmt.Sprintf("%d", limits.BlockSoft),
			fmt.Sprintf("%d", limits.BlockHard),
			fmt.Sprintf("%d", limits.InodeSoft),
			fmt.Sprintf("%d", limits.InodeHard),
			m.mountPoint,
		),
	)
}

func (m *LinuxQuotaManager) GetLimits(uid uint32) (backend.DiskLimits, error) {
	repquota := exec.Command(
		path.Join(m.rootPath, "bin", "repquota"),
		m.mountPoint,
		fmt.Sprintf("%d", uid),
	)

	limits := backend.DiskLimits{}

	out, err := repquota.StdoutPipe()
	if err != nil {
		return limits, err
	}

	err = m.runner.Start(repquota)
	if err != nil {
		return limits, err
	}

	var skip uint32

	_, err = fmt.Fscanf(
		out,
		"%d %d %d %d %d %d %d %d",
		&skip,
		&skip,
		&limits.BlockSoft,
		&limits.BlockHard,
		&skip,
		&skip,
		&limits.InodeSoft,
		&limits.InodeHard,
	)

	if err != nil {
		return limits, err
	}

	return limits, nil
}

func findMountPoint(location string) (string, error) {
	isMount, err := isMountPoint(location)
	if err != nil {
		return "", err
	}

	if isMount {
		return location, nil
	}

	return findMountPoint(path.Dir(location))
}

func isMountPoint(location string) (bool, error) {
	stat, err := os.Stat(location)
	if err != nil {
		return false, err
	}

	parentStat, err := os.Stat(path.Dir(location))
	if err != nil {
		return false, err
	}

	sys := stat.Sys().(*syscall.Stat_t)
	parentSys := parentStat.Sys().(*syscall.Stat_t)

	if sys.Dev != parentSys.Dev {
		return true, nil
	}

	if sys.Ino == parentSys.Ino {
		return true, nil
	}

	return false, nil
}
