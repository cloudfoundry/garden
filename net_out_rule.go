package garden

import "net"

type NetOutRule struct {
	// the protocol to be whitelisted
	Protocol Protocol `json:"protocol,omitempty"`

	// a list of ranges of IP addresses to whitelist; Start to End inclusive; default all
	Networks []IPRange `json:"networks,omitempty"`

	// a list of ranges of ports to whitelist; Start to End inclusive; ignored if Protocol is ICMP; default all
	Ports []PortRange `json:"ports,omitempty"`

	// specifying which ICMP codes to whitelist; ignored if Protocol is not ICMP; default all
	ICMPs *ICMPControl `json:"icmps,omitempty"`

	// if true, logging is enabled; ignored if Protocol is not TCP or All; default false
	Log bool `json:"log,omitempty"`
}

type Protocol uint8

const (
	ProtocolAll Protocol = iota
	ProtocolTCP
	ProtocolUDP
	ProtocolICMP
)

type IPRange struct {
	Start net.IP `json:"start,omitempty"`
	End   net.IP `json:"end,omitempty"`
}

type PortRange struct {
	Start uint16 `json:"start,omitempty"`
	End   uint16 `json:"end,omitempty"`
}

type ICMPType uint8
type ICMPCode uint8

type ICMPControl struct {
	Type ICMPType  `json:"type,omitempty"`
	Code *ICMPCode `json:"code,omitempty"`
}

// IPRangeFromIP creates an IPRange containing a single IP
func IPRangeFromIP(ip net.IP) IPRange {
	return IPRange{Start: ip, End: ip}
}

// IPRangeFromIPNet creates an IPRange containing the same IPs as a given IPNet
func IPRangeFromIPNet(ipNet *net.IPNet) IPRange {
	return IPRange{Start: ipNet.IP, End: lastIP(ipNet)}
}

// PortRangeFromPort creates a PortRange containing a single port
func PortRangeFromPort(port uint16) PortRange {
	return PortRange{Start: port, End: port}
}

// ICMPControlCode creates a value for the Code field in ICMPControl
func ICMPControlCode(code uint8) *ICMPCode {
	pCode := ICMPCode(code)
	return &pCode
}

// Last IP (broadcast) address in a network (net.IPNet)
func lastIP(n *net.IPNet) net.IP {
	mask := n.Mask
	ip := n.IP
	lastip := make(net.IP, len(ip))
	// set bits zero in the mask to ones in ip
	for i, m := range mask {
		lastip[i] = (^m) | ip[i]
	}
	return lastip
}
