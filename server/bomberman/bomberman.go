package bomberman

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server/timebomb"
)

type action int

const (
	strap action = iota
	defuse
	reset
)

type bomb struct {
	Action       action
	Container    garden.Container
	DefuseHandle string
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
	b.bomb <- bomb{Action: strap, Container: container}
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

func (b *Bomberman) Reset(container garden.Container) {
	b.bomb <- bomb{Action: reset, Container: container}
}

func (b *Bomberman) manageBombs() {
	timeBombs := map[string]*timebomb.TimeBomb{}

	for {
		select {
		case bombSignal := <-b.bomb:
			switch bombSignal.Action {
			case strap:
				container := bombSignal.Container

				bomb := timebomb.New(
					b.backend.GraceTime(container),
					func() {
						b.detonate(container)
						b.cleanup <- container.Handle()
					},
				)

				timeBombs[container.Handle()] = bomb
				if b.backend.GraceTime(container) != 0 {
					bomb.Strap()
				}

			case defuse:
				bomb, found := timeBombs[bombSignal.DefuseHandle]
				if !found {
					continue
				}

				bomb.Defuse()

				delete(timeBombs, bombSignal.DefuseHandle)

			case reset:
				container := bombSignal.Container
				bomb, found := timeBombs[container.Handle()]
				if !found {
					continue
				}

				graceTime := b.backend.GraceTime(container)
				bomb.Reset(graceTime)
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
