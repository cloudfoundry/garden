package garden

import "time"

//go:generate counterfeiter . Backend

type Backend interface {
	Client

	Start() error
	Stop()

	GraceTime(Container) time.Duration
}
