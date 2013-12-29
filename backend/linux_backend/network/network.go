package network

import (
	"encoding/json"
	"net"
)

type Network struct {
	ipNet *net.IPNet

	hostIP      net.IP
	containerIP net.IP
}

func New(ipNet *net.IPNet, hostIP, containerIP net.IP) *Network {
	return &Network{
		ipNet:       ipNet,
		hostIP:      hostIP,
		containerIP: containerIP,
	}
}

func (n Network) String() string {
	return n.ipNet.String()
}

func (n Network) IP() net.IP {
	return n.ipNet.IP
}

func (n Network) HostIP() net.IP {
	return n.hostIP
}

func (n Network) ContainerIP() net.IP {
	return n.containerIP
}

func (n Network) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"IPNet": n.String(),

		"HostIP":      n.HostIP(),
		"ContainerIP": n.ContainerIP(),
	})
}

func (n *Network) UnmarshalJSON(data []byte) error {
	var tmp struct {
		IPNet string

		HostIP      net.IP
		ContainerIP net.IP
	}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	_, ipNet, err := net.ParseCIDR(tmp.IPNet)
	if err != nil {
		return err
	}

	n.ipNet = ipNet
	n.hostIP = tmp.HostIP
	n.containerIP = tmp.ContainerIP

	return nil
}
