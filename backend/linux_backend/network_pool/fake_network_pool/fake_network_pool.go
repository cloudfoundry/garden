package fake_network_pool

import (
	"net"

	"github.com/vito/garden/backend/linux_backend/network"
)

type FakeNetworkPool struct {
	ipNet       *net.IPNet
	nextNetwork net.IP

	AcquireError error

	Released []string
}

func New(ipNet *net.IPNet) *FakeNetworkPool {
	return &FakeNetworkPool{
		ipNet: ipNet,

		nextNetwork: ipNet.IP,
	}
}

func (p *FakeNetworkPool) Acquire() (*network.Network, error) {
	if p.AcquireError != nil {
		return nil, p.AcquireError
	}

	_, ipNet, err := net.ParseCIDR(p.nextNetwork.String() + "/30")
	if err != nil {
		return nil, err
	}

	inc(p.nextNetwork)

	hostIP := net.ParseIP(p.nextNetwork.String())
	inc(p.nextNetwork)

	containerIP := net.ParseIP(p.nextNetwork.String())
	inc(p.nextNetwork)

	inc(p.nextNetwork)

	return network.New(ipNet, hostIP, containerIP), nil
}

func (p *FakeNetworkPool) Release(network *network.Network) {
	p.Released = append(p.Released, network.String())
}

func (p *FakeNetworkPool) Network() *net.IPNet {
	return p.ipNet
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
