package timebomb_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden/server/timebomb"
)

var _ = Describe("THE TIMEBOMB", func() {
	var (
		detonated chan time.Time
		countdown time.Duration
		bomb      *timebomb.TimeBomb
	)

	BeforeEach(func() {
		countdown = 100 * time.Millisecond
	})

	JustBeforeEach(func() {
		detonated = make(chan time.Time)

		bomb = timebomb.New(
			countdown,
			func() {
				detonated <- time.Now()
			},
		)
	})

	Context("WHEN STRAPPED", func() {
		It("DETONATES AFTER THE COUNTDOWN", func() {
			before := time.Now()

			bomb.Strap()

			Expect((<-detonated).Sub(before)).To(BeNumerically(">=", countdown))
		})

		It("DOES NOT DETONATE AGAIN", func() {
			before := time.Now()

			bomb.Strap()

			Expect((<-detonated).Sub(before)).To(BeNumerically(">=", countdown))

			delay := 50 * time.Millisecond

			select {
			case <-detonated:
				Fail("MILLIONS ARE DEAD...AGAIN")
			case <-time.After(countdown + delay):
			}
		})

		Context("AND THEN DEFUSED", func() {
			It("DOES NOT DETONATE", func() {
				bomb.Strap()
				bomb.Defuse()

				delay := 50 * time.Millisecond

				select {
				case <-detonated:
					Fail("MILLIONS ARE DEAD")
				case <-time.After(countdown + delay):
				}
			})
		})

		Context("AND THEN PAUSED", func() {
			It("DOES NOT DETONATE", func() {
				bomb.Strap()
				bomb.Pause()

				delay := 50 * time.Millisecond

				select {
				case <-detonated:
					Fail("MILLIONS ARE DEAD")
				case <-time.After(countdown + delay):
				}
			})

			Context("AND THEN DEFUSED", func() {
				It("DOES NOT DETONATE", func() {
					bomb.Strap()
					bomb.Pause()
					bomb.Defuse()

					delay := 50 * time.Millisecond

					select {
					case <-detonated:
						Fail("MILLIONS ARE DEAD")
					case <-time.After(countdown + delay):
					}
				})

				Context("AND THEN UNPAUSED", func() {
					It("DOES NOT DETONATE", func() {
						bomb.Strap()
						bomb.Pause()
						bomb.Defuse()
						bomb.Unpause()

						delay := 50 * time.Millisecond

						select {
						case <-detonated:
							Fail("MILLIONS ARE DEAD")
						case <-time.After(countdown + delay):
						}
					})
				})
			})

			Context("AND THEN UNPAUSED", func() {
				It("DETONATES AFTER THE COUNTDOWN", func() {
					before := time.Now()

					bomb.Strap()

					bomb.Pause()

					delay := 50 * time.Millisecond

					time.Sleep(delay)

					bomb.Unpause()

					Expect((<-detonated).Sub(before)).To(BeNumerically(">=", countdown+delay))
				})

				Context("AND THEN PAUSED AGAIN", func() {
					It("DOES NOT DETONATE", func() {
						bomb.Strap()
						bomb.Pause()
						bomb.Unpause()
						bomb.Pause()

						delay := 50 * time.Millisecond

						select {
						case <-detonated:
							Fail("MILLIONS ARE DEAD")
						case <-time.After(countdown + delay):
						}
					})
				})
			})

			Context("TWICE", func() {
				Context("AND THEN UNPAUSED", func() {
					It("DOES NOT DETONATE", func() {
						bomb.Strap()
						bomb.Pause()
						bomb.Pause()
						bomb.Unpause()

						delay := 50 * time.Millisecond

						select {
						case <-detonated:
							Fail("MILLIONS ARE DEAD")
						case <-time.After(countdown + delay):
						}
					})

					Context("TWICE", func() {
						It("DETONATES AFTER THE COUNTDOWN", func() {
							before := time.Now()

							bomb.Strap()

							bomb.Pause()
							bomb.Pause()

							bomb.Unpause()

							delay := 50 * time.Millisecond

							time.Sleep(delay)

							bomb.Unpause()

							Expect((<-detonated).Sub(before)).To(BeNumerically(">=", countdown+delay))
						})
					})
				})
			})
		})
	})

	Context("WHEN THE COUNTDOWN IS ZERO", func() {
		BeforeEach(func() {
			countdown = 0
		})

		It("DOES NOT EXPLODE ON UNPAUSE", func() {
			bomb.Pause()
			bomb.Unpause()

			select {
			case <-detonated:
				Fail("MILLIONS ARE DEAD")
			case <-time.After(100 * time.Millisecond):
			}
		})

	})

	Context("WHEN RESET", func() {
		It("DOES NOT DETONATE AT THE OLD COUNTDOWN", func() {
			bomb.Strap()

			newCountdown := 10 * time.Second
			bomb.Reset(newCountdown)
			Consistently(func() int {
				return len(detonated)
			}, "1s", "1s").Should(BeZero())
		})

		Context("TO A SHORTER COUNTDOWN", func() {
			BeforeEach(func() {
				countdown = 10 * time.Second
			})
			It("DETONATES AFTER THE NEW COUNTDOWN", func() {
				bomb.Strap()

				newCountdown := 10 * time.Millisecond
				bomb.Reset(newCountdown)
				resetAt := time.Now()
				Expect((<-detonated).Sub(resetAt)).To(BeNumerically("<=", 100*time.Millisecond))
			})
		})
	})
})
