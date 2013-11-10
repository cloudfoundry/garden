package network_pool

import (
	"net"
	"sync"
)

type NetworkPool struct {
	ipNet *net.IPNet

	pool []net.IP

	sync.Mutex
}

type PoolExhaustedError struct{}

func (e PoolExhaustedError) Error() string {
	return "Network pool is exhausted"
}

func New(ipNet *net.IPNet) *NetworkPool {
	pool := []net.IP{}

	for ip := ipNet.IP; ipNet.Contains(ip); ip = nextSubnet(ip) {
		pool = append(pool, ip)
	}

	return &NetworkPool{
		ipNet: ipNet,

		pool: pool,
	}
}

func (p *NetworkPool) Acquire() (net.IP, error) {
	p.Lock()
	defer p.Unlock()

	if len(p.pool) == 0 {
		return nil, PoolExhaustedError{}
	}

	acquired := p.pool[0]
	p.pool = p.pool[1:]

	return acquired, nil
}

func (p *NetworkPool) Release(ip net.IP) {
	if !p.ipNet.Contains(ip) {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.pool = append(p.pool, ip)
}

func nextSubnet(ip net.IP) net.IP {
	next := net.ParseIP(ip.String())

	inc(next)
	inc(next)
	inc(next)
	inc(next)

	return net.IP(next)
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
