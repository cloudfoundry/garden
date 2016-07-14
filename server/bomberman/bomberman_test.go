package bomberman_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"code.cloudfoundry.org/garden"
	fakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server/bomberman"
)

var _ = Describe("Bomberman", func() {
	It("straps a bomb to the given container with the container's grace time as the countdown", func() {
		detonated := make(chan garden.Container)

		backend := new(fakes.FakeBackend)
		backend.GraceTimeReturns(100 * time.Millisecond)

		bomberman := bomberman.New(backend, func(container garden.Container) {
			detonated <- container
		})

		container := new(fakes.FakeContainer)
		container.HandleReturns("doomed")

		bomberman.Strap(container)

		select {
		case <-detonated:
		case <-time.After(backend.GraceTime(container) + 50*time.Millisecond):
			Fail("did not detonate!")
		}
	})

	Context("when the container has a grace time of 0", func() {
		It("never detonates", func() {
			detonated := make(chan garden.Container)

			backend := new(fakes.FakeBackend)
			backend.GraceTimeReturns(0)

			bomberman := bomberman.New(backend, func(container garden.Container) {
				detonated <- container
			})

			container := new(fakes.FakeContainer)
			container.HandleReturns("doomed")

			bomberman.Strap(container)

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(backend.GraceTime(container) + 50*time.Millisecond):
			}
		})
	})

	Describe("pausing a container's timebomb", func() {
		It("prevents it from detonating", func() {
			detonated := make(chan garden.Container)

			backend := new(fakes.FakeBackend)
			backend.GraceTimeReturns(100 * time.Millisecond)

			bomberman := bomberman.New(backend, func(container garden.Container) {
				detonated <- container
			})

			container := new(fakes.FakeContainer)
			container.HandleReturns("doomed")

			bomberman.Strap(container)
			bomberman.Pause("doomed")

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(backend.GraceTime(container) + 50*time.Millisecond):
			}
		})

		Context("when the handle is invalid", func() {
			It("doesn't launch any missiles or anything like that", func() {
				bomberman := bomberman.New(new(fakes.FakeBackend), func(container garden.Container) {
					panic("dont call me")
				})

				bomberman.Pause("BOOM?!")
			})
		})

		Describe("and then unpausing it", func() {
			It("causes it to detonate after the countdown", func() {
				detonated := make(chan garden.Container)

				backend := new(fakes.FakeBackend)
				backend.GraceTimeReturns(100 * time.Millisecond)

				bomberman := bomberman.New(backend, func(container garden.Container) {
					detonated <- container
				})

				container := new(fakes.FakeContainer)
				container.HandleReturns("doomed")

				bomberman.Strap(container)
				bomberman.Pause("doomed")

				before := time.Now()
				bomberman.Unpause("doomed")

				select {
				case <-detonated:
					Ω(time.Since(before)).Should(BeNumerically(">=", 100*time.Millisecond))
				case <-time.After(backend.GraceTime(container) + 50*time.Millisecond):
					Fail("did not detonate!")
				}
			})

			Context("when the handle is invalid", func() {
				It("doesn't launch any missiles or anything like that", func() {
					bomberman := bomberman.New(new(fakes.FakeBackend), func(container garden.Container) {
						panic("dont call me")
					})

					bomberman.Unpause("BOOM?!")
				})
			})
		})
	})

	Describe("defusing a container's timebomb", func() {
		It("prevents it from detonating", func() {
			detonated := make(chan garden.Container)

			backend := new(fakes.FakeBackend)
			backend.GraceTimeReturns(100 * time.Millisecond)

			bomberman := bomberman.New(backend, func(container garden.Container) {
				detonated <- container
			})

			container := new(fakes.FakeContainer)
			container.HandleReturns("doomed")

			bomberman.Strap(container)
			bomberman.Defuse("doomed")

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(backend.GraceTime(container) + 50*time.Millisecond):
			}
		})

		Context("when the handle is invalid", func() {
			It("doesn't launch any missiles or anything like that", func() {
				bomberman := bomberman.New(new(fakes.FakeBackend), func(container garden.Container) {
					panic("dont call me")
				})

				bomberman.Defuse("BOOM?!")
			})
		})
	})
})
