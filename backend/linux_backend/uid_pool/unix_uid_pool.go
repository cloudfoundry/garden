package uid_pool

import (
	"fmt"
	"sync"
)

type UnixUIDPool struct {
	start uint32
	size  uint32

	pool []uint32

	sync.Mutex
}

type PoolExhaustedError struct{}

func (e PoolExhaustedError) Error() string {
	return "UID pool is exhausted"
}

type SystemUserOverlapError struct {
	poolStart uint32
	poolSize  uint32

	user *user.User
}

func (e SystemUserOverlapError) Error() string {
	return fmt.Sprintf(
		"system user overlaps with UID pool (%d-%d): %v",
		e.poolStart,
		e.poolStart+e.poolSize,
		e.user,
	)
}

func New(start, size uint32) *UnixUIDPool {
	pool := []uint32{}

	for i := start; i < start+size; i++ {
		pool = append(pool, i)
	}

	return &UnixUIDPool{
		start: start,
		size:  size,

		pool: pool,
	}
}

func (p *UnixUIDPool) Acquire() (uint32, error) {
	p.Lock()
	defer p.Unlock()

	if len(p.pool) == 0 {
		return 0, PoolExhaustedError{}
	}

	uid := p.pool[0]

	p.pool = p.pool[1:]

	return uid, nil
}

func (p *UnixUIDPool) Release(uid uint32) {
	if uid < p.start || uid >= p.start+p.size {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.pool = append(p.pool, uid)
}
