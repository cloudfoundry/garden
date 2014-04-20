package warden

type Backend interface {
	Client

	Start() error
	Stop()
}
