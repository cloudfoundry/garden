package fake_network_pool

import (
	"net"
)

type FakeNetworkPool struct {
	nextNetwork net.IP

	AcquireError error

	Released []string
}

func New(start net.IP) *FakeNetworkPool {
	return &FakeNetworkPool{
		nextNetwork: start,
	}
}

func (p *FakeNetworkPool) Acquire() (net.IP, error) {
	if p.AcquireError != nil {
		return nil, p.AcquireError
	}

	ip := net.ParseIP(p.nextNetwork.String())

	inc(p.nextNetwork)

	return ip, nil
}

func (p *FakeNetworkPool) Release(ip net.IP) {
	p.Released = append(p.Released, ip.String())
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
