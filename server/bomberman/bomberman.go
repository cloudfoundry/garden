package bomberman

import (
	"github.com/cloudfoundry-incubator/garden/server/timebomb"
	"github.com/cloudfoundry-incubator/garden/warden"
)

type Bomberman struct {
	detonate func(warden.BackendContainer)

	strap   chan warden.BackendContainer
	pause   chan string
	unpause chan string
	defuse  chan string
}

func New(detonate func(warden.BackendContainer)) *Bomberman {
	b := &Bomberman{
		detonate: detonate,

		strap:   make(chan warden.BackendContainer),
		pause:   make(chan string),
		unpause: make(chan string),
		defuse:  make(chan string),
	}

	go b.manageBombs()

	return b
}

func (b *Bomberman) Strap(container warden.BackendContainer) {
	b.strap <- container
}

func (b *Bomberman) Pause(name string) {
	b.pause <- name
}

func (b *Bomberman) Unpause(name string) {
	b.unpause <- name
}

func (b *Bomberman) Defuse(name string) {
	b.defuse <- name
}

func (b *Bomberman) manageBombs() {
	timeBombs := map[string]*timebomb.TimeBomb{}

	for {
		select {
		case container := <-b.strap:
			if container.GraceTime() == 0 {
				continue
			}

			bomb := timebomb.New(
				container.GraceTime(),
				func() {
					b.detonate(container)
					b.defuse <- container.Handle()
				},
			)

			timeBombs[container.Handle()] = bomb

			bomb.Strap()

		case handle := <-b.pause:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Pause()

		case handle := <-b.unpause:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Unpause()

		case handle := <-b.defuse:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Defuse()

			delete(timeBombs, handle)
		}
	}
}
