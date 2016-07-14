package bomberman

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server/timebomb"
)

type action int

const (
	strap action = iota
	defuse
)

type bomb struct {
	Action         action
	StrapContainer garden.Container
	DefuseHandle   string
}

type Bomberman struct {
	backend garden.Backend

	detonate func(garden.Container)

	pause   chan string
	unpause chan string
	cleanup chan string
	bomb    chan bomb
}

func New(backend garden.Backend, detonate func(garden.Container)) *Bomberman {
	b := &Bomberman{
		backend:  backend,
		detonate: detonate,

		bomb:    make(chan bomb),
		pause:   make(chan string),
		unpause: make(chan string),
		cleanup: make(chan string),
	}

	go b.manageBombs()

	return b
}

func (b *Bomberman) Strap(container garden.Container) {
	b.bomb <- bomb{Action: strap, StrapContainer: container}
}

func (b *Bomberman) Pause(name string) {
	b.pause <- name
}

func (b *Bomberman) Unpause(name string) {
	b.unpause <- name
}

func (b *Bomberman) Defuse(name string) {
	b.bomb <- bomb{Action: defuse, DefuseHandle: name}
}

func (b *Bomberman) manageBombs() {
	timeBombs := map[string]*timebomb.TimeBomb{}

	for {
		select {
		case bombSignal := <-b.bomb:
			switch bombSignal.Action {
			case strap:
				container := bombSignal.StrapContainer

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

			case defuse:
				bomb, found := timeBombs[bombSignal.DefuseHandle]
				if !found {
					continue
				}

				bomb.Defuse()

				delete(timeBombs, bombSignal.DefuseHandle)
			}
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

		case handle := <-b.cleanup:
			delete(timeBombs, handle)
		}
	}
}
