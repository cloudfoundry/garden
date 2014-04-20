package warden

import "time"

type Backend interface {
	Client

	Start() error
	Stop()

	GraceTime(Container) time.Duration
}
