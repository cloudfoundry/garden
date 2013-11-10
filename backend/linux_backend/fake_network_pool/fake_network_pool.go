package fake_network_pool

import (
	"net"

	"github.com/vito/garden/backend/linux_backend/network"
)

type FakeNetworkPool struct {
	nextNetwork net.IP

	AcquireError error

	Released []string
}

type FakeNetwork struct {
	network     net.IP
	hostIP      net.IP
	containerIP net.IP
}

func (n FakeNetwork) String() string {
	return n.network.String() + "/30"
}

func (n FakeNetwork) HostIP() net.IP {
	return n.hostIP
}

func (n FakeNetwork) ContainerIP() net.IP {
	return n.containerIP
}

func New(start net.IP) *FakeNetworkPool {
	return &FakeNetworkPool{
		nextNetwork: start,
	}
}

func (p *FakeNetworkPool) Acquire() (network.Network, error) {
	if p.AcquireError != nil {
		return nil, p.AcquireError
	}

	network := net.ParseIP(p.nextNetwork.String())
	inc(p.nextNetwork)

	hostIP := net.ParseIP(p.nextNetwork.String())
	inc(p.nextNetwork)

	containerIP := net.ParseIP(p.nextNetwork.String())
	inc(p.nextNetwork)

	inc(p.nextNetwork)

	return &FakeNetwork{
		network:     network,
		hostIP:      hostIP,
		containerIP: containerIP,
	}, nil
}

func (p *FakeNetworkPool) Release(network network.Network) {
	p.Released = append(p.Released, network.String())
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
