package backend

type Backend interface {
	Create(ContainerSpec) (Container, error)
	Containers() ([]Container, error)
}
