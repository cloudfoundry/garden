package fake_bandwidth_manager

import (
	"github.com/vito/garden/backend"
)

type FakeBandwidthManager struct {
	SetLimitsError error
	EnforcedLimits []backend.BandwidthLimits

	GetUsageError  error
	GetUsageResult backend.ContainerBandwidthStat
}

func New() *FakeBandwidthManager {
	return &FakeBandwidthManager{}
}

func (m *FakeBandwidthManager) SetLimits(limits backend.BandwidthLimits) error {
	if m.SetLimitsError != nil {
		return m.SetLimitsError
	}

	m.EnforcedLimits = append(m.EnforcedLimits, limits)

	return nil
}

func (m *FakeBandwidthManager) GetUsage() (backend.ContainerBandwidthStat, error) {
	if m.GetUsageError != nil {
		return backend.ContainerBandwidthStat{}, m.GetUsageError
	}

	return m.GetUsageResult, nil
}
