package transport

import "github.com/cloudfoundry-incubator/garden"

type Source int

const (
	Stdin Source = iota
	Stdout
	Stderr
)

type ProcessPayload struct {
	ProcessID  uint32          `json:"process_id,omitempty"`
	StreamID   string          `json:"stream_id,omitempty"`
	Source     *Source         `json:"source,omitempty"`
	Data       *string         `json:"data,omitempty"`
	ExitStatus *int            `json:"exit_status,omitempty"`
	Error      *string         `json:"error,omitempty"`
	TTY        *garden.TTYSpec `json:"tty,omitempty"`
	Signal     *garden.Signal  `json:"signal,omitempty"`
}

type NetInRequest struct {
	Handle        string `json:"handle,omitempty"`
	HostPort      uint32 `json:"host_port,omitempty"`
	ContainerPort uint32 `json:"container_port,omitempty"`
}

type NetInResponse struct {
	HostPort      uint32 `json:"host_port,omitempty"`
	ContainerPort uint32 `json:"container_port,omitempty"`
}
