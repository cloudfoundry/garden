package bandwidth_manager

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/vito/garden/backend"
	"github.com/vito/garden/command_runner"
)

type BandwidthManager interface {
	SetLimits(backend.BandwidthLimits) error
	GetUsage() (backend.ContainerBandwidthStat, error)
}

type ContainerBandwidthManager struct {
	containerPath string
	runner        command_runner.CommandRunner
}

func New(containerPath string, runner command_runner.CommandRunner) *ContainerBandwidthManager {
	return &ContainerBandwidthManager{
		containerPath: containerPath,
		runner:        runner,
	}
}

func (m *ContainerBandwidthManager) SetLimits(limits backend.BandwidthLimits) error {
	limit := exec.Command(path.Join(m.containerPath, "net_rate.sh"))

	limit.Env = []string{
		fmt.Sprintf("BURST=%d", limits.BurstRateInBytesPerSecond),
		fmt.Sprintf("RATE=%d", limits.RateInBytesPerSecond*8),
	}

	return m.runner.Run(limit)
}

func (m *ContainerBandwidthManager) GetUsage() (backend.ContainerBandwidthStat, error) {
	return backend.ContainerBandwidthStat{}, nil
}
