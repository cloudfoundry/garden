package garden

type NetIn struct {
	// Host port from which to forward traffic to the container
	HostPort uint32 `json:"host_port"`

	// Container port to which host traffic will be forwarded
	ContainerPort uint32 `json:"container_port"`
}
