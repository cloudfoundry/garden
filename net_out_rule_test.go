package garden_test

import (
	"net"

	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NetOutRule helper functions", func() {
	Describe("IPRangeFromIP", func() {
		It("Creates an IPRange with the Start and End set to the passed IP", func() {
			r := garden.IPRangeFromIP(net.ParseIP("1.2.3.4"))
			Ω(r.Start).Should(Equal(net.ParseIP("1.2.3.4")))
			Ω(r.End).Should(Equal(r.Start))
		})
	})

	Describe("IPRangeFromIPNet", func() {
		It("Creates an IPRange with the Start and End set to the extent of the IPNet", func() {
			ip, cidr, err := net.ParseCIDR("1.2.3.0/24")
			Ω(err).Should(Succeed())

			r := garden.IPRangeFromIPNet(cidr)
			Ω(r.Start.String()).Should(Equal(ip.String()))
			Ω(r.End.String()).Should(Equal("1.2.3.255"))
		})
	})

	Describe("PortRangeFromPort", func() {
		It("Creates an PortRange with the Start and End set to the passed port", func() {
			r := garden.PortRangeFromPort(2)
			Ω(r.Start).Should(Equal(uint16(2)))
			Ω(r.End).Should(Equal(r.Start))
		})
	})

	Describe("ICMPControlCode", func() {
		It("returns an ICMPCode with the passed uint8", func() {
			var icmpVar *garden.ICMPCode
			code := garden.ICMPControlCode(uint8(2))
			Ω(code).Should(BeAssignableToTypeOf(icmpVar))
			Ω(*code).Should(BeNumerically("==", 2))
		})
	})
})
