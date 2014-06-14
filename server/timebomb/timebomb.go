package timebomb

import (
	"sync"
	"time"
)

type TimeBomb struct {
	countdown time.Duration
	detonate  func()

	pauses  int
	defused bool
	timer   *time.Timer
	lock    *sync.Mutex
}

func New(countdown time.Duration, detonate func()) *TimeBomb {
	return &TimeBomb{
		countdown: countdown,
		detonate:  detonate,

		lock: new(sync.Mutex),
	}
}

func (b *TimeBomb) Strap() {
	b.lock.Lock()
	b.timer = time.AfterFunc(b.countdown, b.detonate)
	b.lock.Unlock()
}

func (b *TimeBomb) Pause() bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	timer := b.timer
	b.timer = nil

	b.pauses++

	if timer == nil {
		return true
	}

	if !timer.Stop() {
		return false
	}

	return true
}

func (b *TimeBomb) Defuse() bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.defused = true

	timer := b.timer
	b.timer = nil

	if timer == nil {
		return true
	}

	return timer.Stop()
}

func (b *TimeBomb) Unpause() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.pauses--

	if !b.defused && b.pauses == 0 {
		b.timer = time.AfterFunc(b.countdown, b.detonate)
	}
}
