package timebomb

import (
	"sync"
	"time"
)

type TimeBomb struct {
	countdown time.Duration
	detonate  func()

	reset    chan bool
	defuse   chan bool
	cooldown *sync.WaitGroup
}

func New(countdown time.Duration, detonate func()) *TimeBomb {
	return &TimeBomb{
		countdown: countdown,
		detonate:  detonate,

		reset:    make(chan bool),
		defuse:   make(chan bool),
		cooldown: &sync.WaitGroup{},
	}
}

func (b *TimeBomb) Strap() {
	b.cooldown.Add(1)

	go func() {
		for {
			cool := b.waitForCooldown()
			if !cool {
				continue
			}

			select {
			case <-time.After(b.countdown):
				b.detonate()
			case <-b.reset:
			case <-b.defuse:
				return
			}
		}
	}()

	go b.cooldown.Done()
}

func (b *TimeBomb) Pause() {
	b.cooldown.Add(1)
	b.reset <- true
}

func (b *TimeBomb) Defuse() {
	b.defuse <- true
}

func (b *TimeBomb) Unpause() {
	b.cooldown.Done()
}

func (b *TimeBomb) waitForCooldown() bool {
	ready := make(chan bool, 1)

	go func() {
		b.cooldown.Wait()
		ready <- true
	}()

	select {
	case <-ready:
		return true
	case <-b.reset:
		return false
	}
}
