package bomberman

import (
	"github.com/cloudfoundry-incubator/garden/api"
	"github.com/cloudfoundry-incubator/garden/server/timebomb"
)

type Bomberman struct {
	backend api.Backend

	detonate func(api.Container)

	strap   chan api.Container
	pause   chan string
	unpause chan string
	defuse  chan string
	cleanup chan string
}

func New(backend api.Backend, detonate func(api.Container)) *Bomberman {
	b := &Bomberman{
		backend:  backend,
		detonate: detonate,

		strap:   make(chan api.Container),
		pause:   make(chan string),
		unpause: make(chan string),
		defuse:  make(chan string),
		cleanup: make(chan string),
	}

	go b.manageBombs()

	return b
}

func (b *Bomberman) Strap(container api.Container) {
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
			if b.backend.GraceTime(container) == 0 {
				continue
			}

			bomb := timebomb.New(
				b.backend.GraceTime(container),
				func() {
					b.detonate(container)
					b.cleanup <- container.Handle()
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

		case handle := <-b.cleanup:
			delete(timeBombs, handle)
		}
	}
}
