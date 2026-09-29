// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package routeadvertisements

import (
	"net"
	"testing"
)

func TestGenerateRawConfig(t *testing.T) {
	tests := []struct {
		name         string
		selected     *selectedNetworks
		vrfNeighbors map[string][]string
		vrfASNs      map[string]uint32
		want         string
	}{
		{
			name:         "empty input",
			selected:     &selectedNetworks{},
			vrfNeighbors: map[string][]string{},
			vrfASNs:      map[string]uint32{},
			want:         "",
		},
		{
			name:         "default and non-default VRF unicast only",
			selected:     &selectedNetworks{},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1", "fd00::1"}, "red": {"10.0.0.1", "fd00::2"}},
			vrfASNs:      map[string]uint32{"": 65000, "red": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family ipv6 unicast
  neighbor fd00::1 allowas-in origin
 exit-address-family
exit
!
router bgp 65000 vrf red
 address-family ipv4 unicast
  neighbor 10.0.0.1 allowas-in origin
 exit-address-family
 address-family ipv6 unicast
  neighbor fd00::2 allowas-in origin
 exit-address-family
exit
!
`,
		},
		{
			name:         "unnumbered neighbor identified by interface name",
			selected:     &selectedNetworks{},
			vrfNeighbors: map[string][]string{"": {"enp4s0f0np0", "192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 4200000001},
			want: `router bgp 4200000001
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
  neighbor enp4s0f0np0 allowas-in origin
 exit-address-family
 address-family ipv6 unicast
  neighbor enp4s0f0np0 allowas-in origin
 exit-address-family
exit
!
`,
		},
		{
			name: "MAC-VRF without route target",
			selected: &selectedNetworks{
				macVRFConfigs: []*vrfConfig{
					{VNI: 1000},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  advertise-all-vni
 exit-address-family
exit
!
`,
		},
		{
			name: "MAC-VRF with route target",
			selected: &selectedNetworks{
				macVRFConfigs: []*vrfConfig{
					{VNI: 1000, RouteTarget: "65000:1000"},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  advertise-all-vni
  vni 1000
   route-target import 65000:1000
   route-target export 65000:1000
  exit-vni
 exit-address-family
exit
!
`,
		},
		{
			name: "IP-VRF IPv6",
			selected: &selectedNetworks{
				ipVRFConfigs: []*ipVRFConfig{
					{
						vrfConfig: vrfConfig{VNI: 2000, RouteTarget: "65000:2000"},
						VRFName:   "blue",
						HasIPv6:   true,
					},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 65000, "blue": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  advertise-all-vni
 exit-address-family
exit
!
vrf blue
 vni 2000
exit-vrf
!
router bgp 65000 vrf blue
 address-family l2vpn evpn
  advertise ipv6 unicast
  route-target import 65000:2000
  route-target export 65000:2000
 exit-address-family
exit
!
`,
		},
		{
			name: "IP-VRF dual stack",
			selected: &selectedNetworks{
				ipVRFConfigs: []*ipVRFConfig{
					{
						vrfConfig: vrfConfig{VNI: 2000, RouteTarget: "65000:2000"},
						VRFName:   "blue",
						HasIPv4:   true,
						HasIPv6:   true,
					},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 65000, "blue": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  advertise-all-vni
 exit-address-family
exit
!
vrf blue
 vni 2000
exit-vrf
!
router bgp 65000 vrf blue
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
  route-target import 65000:2000
  route-target export 65000:2000
 exit-address-family
exit
!
`,
		},
		{
			name: "MAC-VRF and IP-VRF combined",
			selected: &selectedNetworks{
				macVRFConfigs: []*vrfConfig{
					{VNI: 1000, RouteTarget: "65000:1000"},
				},
				ipVRFConfigs: []*ipVRFConfig{
					{
						vrfConfig: vrfConfig{VNI: 2000, RouteTarget: "65000:2000"},
						VRFName:   "blue",
						HasIPv4:   true,
					},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.2", "192.168.1.1"}},
			vrfASNs:      map[string]uint32{"": 65000, "blue": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
  neighbor 192.168.1.2 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  neighbor 192.168.1.2 activate
  neighbor 192.168.1.2 allowas-in origin
  advertise-all-vni
  vni 1000
   route-target import 65000:1000
   route-target export 65000:1000
  exit-vni
 exit-address-family
exit
!
vrf blue
 vni 2000
exit-vrf
!
router bgp 65000 vrf blue
 address-family l2vpn evpn
  advertise ipv4 unicast
  route-target import 65000:2000
  route-target export 65000:2000
 exit-address-family
exit
!
`,
		},
		{
			name: "non-default VRF with neighbors and IP-VRF",
			selected: &selectedNetworks{
				ipVRFConfigs: []*ipVRFConfig{
					{
						vrfConfig: vrfConfig{VNI: 3000, RouteTarget: "65000:3000"},
						VRFName:   "green",
						HasIPv4:   true,
					},
				},
			},
			vrfNeighbors: map[string][]string{"": {"192.168.1.1"}, "green": {"10.0.0.1"}},
			vrfASNs:      map[string]uint32{"": 65000, "green": 65000},
			want: `router bgp 65000
 address-family ipv4 unicast
  neighbor 192.168.1.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  neighbor 192.168.1.1 activate
  neighbor 192.168.1.1 allowas-in origin
  advertise-all-vni
 exit-address-family
exit
!
vrf green
 vni 3000
exit-vrf
!
router bgp 65000 vrf green
 address-family ipv4 unicast
  neighbor 10.0.0.1 allowas-in origin
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  route-target import 65000:3000
  route-target export 65000:3000
 exit-address-family
exit
!
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRawConfig(tt.selected, tt.vrfNeighbors, tt.vrfASNs)
			if got != tt.want {
				t.Errorf("generateRawConfig() mismatch\nGot:\n%s\nWant:\n%s", got, tt.want)
			}
		})
	}
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestGenCNCParentVRFSection(t *testing.T) {
	tests := []struct {
		name            string
		parentVRFName   string
		asn             uint32
		l3VNI           int32
		childVRFNames   []string
		allChildSubnets []cncSubnet
		want            string
	}{
		{
			name:          "zero VNI returns empty",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         0,
			childVRFNames: []string{"blue-udn-vrf"},
			want:          "",
		},
		{
			name:          "zero ASN returns empty",
			parentVRFName: "cnc-tenant-vrf",
			asn:           0,
			l3VNI:         5000,
			childVRFNames: []string{"blue-udn-vrf"},
			want:          "",
		},
		{
			name:          "no children — no import lines",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         5000,
			childVRFNames: nil,
			want: `vrf cnc-tenant-vrf
 vni 5000
exit-vrf
!
router bgp 65000 vrf cnc-tenant-vrf
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
		{
			name:          "one child, no subnets — no import route-map, bare import vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         5000,
			childVRFNames: []string{"blue-udn-vrf"},
			want: `vrf cnc-tenant-vrf
 vni 5000
exit-vrf
!
router bgp 65000 vrf cnc-tenant-vrf
 address-family ipv4 unicast
  import vrf blue-udn-vrf
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
		{
			name:          "one Layer3 child with subnet — prefix-list caps at host subnet length",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         5000,
			childVRFNames: []string{"blue-udn-vrf"},
			allChildSubnets: []cncSubnet{
				{CIDR: mustParseCIDR("10.1.0.0/16"), HostSubnetLength: 24},
			},
			want: `ip prefix-list CNC-CNC-TENANT-VRF-PREFIXES seq 10 permit 10.1.0.0/16 le 24
!
route-map CNC-CNC-TENANT-VRF-IMPORT permit 10
 match ip address prefix-list CNC-CNC-TENANT-VRF-PREFIXES
route-map CNC-CNC-TENANT-VRF-IMPORT deny 20
!
vrf cnc-tenant-vrf
 vni 5000
exit-vrf
!
router bgp 65000 vrf cnc-tenant-vrf
 address-family ipv4 unicast
  import vrf route-map CNC-CNC-TENANT-VRF-IMPORT
  import vrf blue-udn-vrf
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
		{
			name:          "two children sorted, no subnets — bare import vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         5000,
			childVRFNames: []string{"green-udn-vrf", "blue-udn-vrf"}, // unsorted input
			want: `vrf cnc-tenant-vrf
 vni 5000
exit-vrf
!
router bgp 65000 vrf cnc-tenant-vrf
 address-family ipv4 unicast
  import vrf blue-udn-vrf
  import vrf green-udn-vrf
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
		{
			name:          "two children, Layer3 and Layer2 subnets sorted — L3 caps at host length, L2 stays exact",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			l3VNI:         5000,
			childVRFNames: []string{"green-udn-vrf", "blue-udn-vrf"}, // unsorted input
			allChildSubnets: []cncSubnet{
				{CIDR: mustParseCIDR("10.2.0.0/24"), HostSubnetLength: 0},  // green Layer2 (unsorted input)
				{CIDR: mustParseCIDR("10.1.0.0/16"), HostSubnetLength: 24}, // blue Layer3
			},
			want: `ip prefix-list CNC-CNC-TENANT-VRF-PREFIXES seq 10 permit 10.1.0.0/16 le 24
ip prefix-list CNC-CNC-TENANT-VRF-PREFIXES seq 20 permit 10.2.0.0/24
!
route-map CNC-CNC-TENANT-VRF-IMPORT permit 10
 match ip address prefix-list CNC-CNC-TENANT-VRF-PREFIXES
route-map CNC-CNC-TENANT-VRF-IMPORT deny 20
!
vrf cnc-tenant-vrf
 vni 5000
exit-vrf
!
router bgp 65000 vrf cnc-tenant-vrf
 address-family ipv4 unicast
  import vrf route-map CNC-CNC-TENANT-VRF-IMPORT
  import vrf blue-udn-vrf
  import vrf green-udn-vrf
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genCNCParentVRFSection(tt.parentVRFName, tt.asn, tt.l3VNI, tt.childVRFNames, tt.allChildSubnets)
			if got != tt.want {
				t.Errorf("genCNCParentVRFSection() mismatch\nGot:\n%s\nWant:\n%s", got, tt.want)
			}
		})
	}
}

func TestGenCNCIPVRFChildSection(t *testing.T) {
	tests := []struct {
		name          string
		childVRFName  string
		parentVRFName string
		asn           uint32
		childSubnets  []cncSubnet
		want          string
	}{
		{
			name:          "zero ASN returns empty",
			childVRFName:  "blue-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           0,
			childSubnets:  []cncSubnet{{CIDR: mustParseCIDR("10.1.0.0/24")}},
			want:          "",
		},
		{
			name:          "IP-VRF child with one subnet",
			childVRFName:  "blue-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			childSubnets:  []cncSubnet{{CIDR: mustParseCIDR("10.1.0.0/24"), HostSubnetLength: 24}},
			want: `ip prefix-list CNC-BLUE-UDN-VRF-PREFIXES seq 10 permit 10.1.0.0/24 le 32
!
route-map CNC-BLUE-UDN-VRF-ADVERTISE permit 10
 match ip address prefix-list CNC-BLUE-UDN-VRF-PREFIXES
!
router bgp 65000 vrf blue-udn-vrf
 address-family l2vpn evpn
  advertise ipv4 unicast route-map CNC-BLUE-UDN-VRF-ADVERTISE
 exit-address-family
exit
!
`,
		},
		{
			name:          "IP-VRF child with two subnets sorted",
			childVRFName:  "blue-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			childSubnets: []cncSubnet{
				{CIDR: mustParseCIDR("10.2.0.0/24"), HostSubnetLength: 24},
				{CIDR: mustParseCIDR("10.1.0.0/24"), HostSubnetLength: 24},
			}, // unsorted input
			want: `ip prefix-list CNC-BLUE-UDN-VRF-PREFIXES seq 10 permit 10.1.0.0/24 le 32
ip prefix-list CNC-BLUE-UDN-VRF-PREFIXES seq 20 permit 10.2.0.0/24 le 32
!
route-map CNC-BLUE-UDN-VRF-ADVERTISE permit 10
 match ip address prefix-list CNC-BLUE-UDN-VRF-PREFIXES
!
router bgp 65000 vrf blue-udn-vrf
 address-family l2vpn evpn
  advertise ipv4 unicast route-map CNC-BLUE-UDN-VRF-ADVERTISE
 exit-address-family
exit
!
`,
		},
		{
			name:          "IP-VRF child with no subnets — bare advertise, no route-map",
			childVRFName:  "blue-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			childSubnets:  nil,
			want: `router bgp 65000 vrf blue-udn-vrf
 address-family l2vpn evpn
  advertise ipv4 unicast
 exit-address-family
exit
!
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genCNCIPVRFChildSection(tt.childVRFName, tt.parentVRFName, tt.asn, tt.childSubnets)
			if got != tt.want {
				t.Errorf("genCNCIPVRFChildSection() mismatch\nGot:\n%s\nWant:\n%s", got, tt.want)
			}
		})
	}
}

func TestGenCNCMACVRFChildSection(t *testing.T) {
	tests := []struct {
		name          string
		childVRFName  string
		parentVRFName string
		asn           uint32
		childSubnets  []cncSubnet
		want          string
	}{
		{
			name:          "zero ASN returns empty",
			childVRFName:  "beta-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           0,
			childSubnets:  []cncSubnet{{CIDR: mustParseCIDR("10.20.0.0/16")}},
			want:          "",
		},
		{
			// MAC-VRF (Layer2) child: the CNC parent VRF acts as its IP-VRF. The child
			// only redistributes connected routes and advertises them; it does not
			// "import vrf <parent>" (that would leak the parent's sibling routes via the
			// overlay next-hop). childSubnets is unused here and does not affect output.
			name:          "Layer2 MAC-VRF child only redistributes connected, no import vrf",
			childVRFName:  "beta-udn-vrf",
			parentVRFName: "cnc-tenant-vrf",
			asn:           65000,
			childSubnets:  []cncSubnet{{CIDR: mustParseCIDR("10.20.0.0/16"), HostSubnetLength: 0}},
			want: `router bgp 65000 vrf beta-udn-vrf
 address-family ipv4 unicast
  redistribute connected
 exit-address-family
 address-family ipv6 unicast
  redistribute connected
 exit-address-family
 address-family l2vpn evpn
  advertise ipv4 unicast
  advertise ipv6 unicast
 exit-address-family
exit
!
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genCNCMACVRFChildSection(tt.childVRFName, tt.parentVRFName, tt.asn, tt.childSubnets)
			if got != tt.want {
				t.Errorf("genCNCMACVRFChildSection() mismatch\nGot:\n%s\nWant:\n%s", got, tt.want)
			}
		})
	}
}
