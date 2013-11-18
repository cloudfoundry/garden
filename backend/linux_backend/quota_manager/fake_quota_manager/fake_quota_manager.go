package fake_quota_manager

import (
	"sync"

	"github.com/vito/garden/backend"
)

type FakeQuotaManager struct {
	SetLimitsError error
	GetLimitsError error

	GetLimitsResult backend.DiskLimits

	Limited map[uint32]backend.DiskLimits

	sync.RWMutex
}

func New() *FakeQuotaManager {
	return &FakeQuotaManager{
		Limited: make(map[uint32]backend.DiskLimits),
	}
}

func (m *FakeQuotaManager) SetLimits(uid uint32, limits backend.DiskLimits) error {
	if m.SetLimitsError != nil {
		return m.SetLimitsError
	}

	m.Lock()
	defer m.Unlock()

	m.Limited[uid] = limits

	return nil
}

func (m *FakeQuotaManager) GetLimits(uid uint32) (backend.DiskLimits, error) {
	if m.GetLimitsError != nil {
		return backend.DiskLimits{}, m.GetLimitsError
	}

	m.RLock()
	defer m.RUnlock()

	return m.GetLimitsResult, nil
}
