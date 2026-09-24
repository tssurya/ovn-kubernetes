// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package routeadvertisements

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"

	utilnet "k8s.io/utils/net"
)

// cncIPVRFChild holds generation data for one IP-VRF child of an EVPN CNC parent VRF.
type cncIPVRFChild struct {
	// VRFName is the Linux VRF name of the child (from GetNetworkVRFName, ≤15 chars).
	VRFName string
	// Subnets are the child network's pod subnets, used by the route-map in
	// genCNCIPVRFChildImportSection to prevent re-advertisement of routes
	// imported from the parent VRF back into EVPN.
	Subnets []*net.IPNet
}

// cncParentVRFConfig holds the FRR generation data for one EVPN CNC parent VRF.
// The parent VRF acts as a routing hub: it owns a single L3 VNI and imports
// routes from all connected child VRFs so they can exchange traffic via EVPN.
type cncParentVRFConfig struct {
	// ParentVRFName is the Linux interface name (≤15 chars, from GetCNCParentVRFName).
	ParentVRFName string
	// ParentVNI is the L3 VNI assigned to the parent VRF.
	ParentVNI int32
	// ChildVRFNames is the ordered list of child VRF names covered by this RA,
	// used in the parent "router bgp" "import vrf" stanzas. When multiple RAs
	// each cover a subset of the CNC's children, each RA emits only its own
	// subset here; FRR applies the partial stanzas additively.
	ChildVRFNames []string
	// IPVRFChildren are IP-VRF children that require raw FRR config to prevent
	// re-advertisement of routes received from the parent VRF back into EVPN.
	IPVRFChildren []*cncIPVRFChild
	// MACVRFOnlyChildVRFNames are MAC-VRF-only children (no IP-VRF) that get a
	// structured frr-k8s Router.Imports entry instead of raw config.
	MACVRFOnlyChildVRFNames []string
}

// generateRawConfig generates raw FRR configuration. Main purpose is to
// generates EVPN config sections based on the provided selected networks
// IP-VRFs and MAC-VRFs. Also adds allowas-in origin to all neighbors, which is
// needed for eBGP peers sharing our ASN and is a no-op for iBGP.
//
// Generated config structure:
//
//	router bgp <asn>                    <- genDefaultVRFSection
//	 address-family ipv4 unicast        <- genUnicastSection (only if default VRF has neighbors)
//	  neighbor <ip> allowas-in origin
//	 exit-address-family
//	 address-family ipv6 unicast
//	  neighbor <ip> allowas-in origin
//	 exit-address-family
//	 address-family l2vpn evpn          <- genDefaultVRFEVPNSection (only if networks have MAC-VRFs/IP-VRFs)
//	  neighbor <ip> activate
//	  neighbor <ip> allowas-in origin
//	  advertise-all-vni
//	  vni <id>                          <- genVRFVNISection (one per MAC-VRF with RT, section only added when MAC-VRF RT is set)
//	   route-target import <rt>
//	   route-target export <rt>
//	  exit-vni
//	 exit-address-family
//	exit
//	!
//	vrf <name>                          <- genVRFVNISection (one per IP-VRF)
//	 vni <id>
//	exit-vrf
//	!
//	router bgp <asn> vrf <name>         <- genNonDefaultVRFSection (one per non-default VRF)
//	 address-family ipv4 unicast        <- genUnicastSection (only if VRF has neighbors i.e. VRF-Lite)
//	  neighbor <ip> allowas-in origin
//	 exit-address-family
//	 address-family ipv6 unicast
//	  neighbor <ip> allowas-in origin
//	 exit-address-family
//	 address-family l2vpn evpn          <- genNonDefaultVRFEVPNSection (only if network has IP-VRF)
//	  advertise ipv4 unicast
//	  advertise ipv6 unicast
//	  route-target import <rt>
//	  route-target export <rt>
//	 exit-address-family
//	exit
//	!
func generateRawConfig(selected *selectedNetworks, vrfNeighbors map[string][]string, vrfASNs map[string]uint32) string {
	var buf strings.Builder

	// handle default VRF router, neighbors sorted for deterministic config
	// generation
	neighbors := slices.Sorted(slices.Values(vrfNeighbors[""]))
	buf.WriteString(genDefaultVRFSection(vrfASNs[""], neighbors, selected))

	// handle VRF<->VNI mappings
	ipVRFConfigMap := make(map[string]*ipVRFConfig, len(selected.ipVRFConfigs))
	for _, cfg := range selected.ipVRFConfigs {
		ipVRFConfigMap[cfg.VRFName] = cfg
		buf.WriteString(genVRFVNISection(cfg))
	}

	// handle non default VRFs
	for _, vrf := range slices.Sorted(maps.Keys(vrfASNs)) {
		if vrf == "" {
			continue
		}
		// neighbors sorted for deterministic config generation
		neighbors := slices.Sorted(slices.Values(vrfNeighbors[vrf]))
		buf.WriteString(genNonDefaultVRFSection(vrf, vrfASNs[vrf], neighbors, ipVRFConfigMap[vrf]))
	}

	// CNC parent VRF sections: one parent VRF per EVPN CNC connecting EVPN-type
	// CUDNs selected by this RA. The parent VRF owns the L3 VNI and imports
	// routes from all child VRFs. IP-VRF children additionally need a raw
	// route-map stanza to prevent re-advertisement of imported routes.
	// MAC-VRF-only children need a dedicated BGP VRF stanza with redistribute
	// connected so their subnets enter the BGP RIB and can be imported by the
	// parent and advertised as EVPN type-5 routes.
	globalASN := vrfASNs[""]
	for _, cncCfg := range selected.cncParentVRFConfigs {
		buf.WriteString(genCNCParentVRFSection(cncCfg.ParentVRFName, globalASN, cncCfg.ParentVNI, cncCfg.ChildVRFNames))
		for _, child := range cncCfg.IPVRFChildren {
			buf.WriteString(genCNCIPVRFChildImportSection(child.VRFName, cncCfg.ParentVRFName, globalASN, child.Subnets))
		}
		for _, childVRF := range cncCfg.MACVRFOnlyChildVRFNames {
			buf.WriteString(genCNCMACVRFChildSection(childVRF, cncCfg.ParentVRFName, globalASN))
		}
	}

	return buf.String()
}

// genVRFVNISection generates VRF-to-VNI mapping.
//
//	vrf <name>
//	 vni <id>
//	exit-vrf
//	!
func genVRFVNISection(cfg *ipVRFConfig) string {
	return fmt.Sprintf(`vrf %s
 vni %d
exit-vrf
!
`, cfg.VRFName, cfg.VNI)
}

// genDefaultVRFSection generates the default VRF router's unicast and EVPN address-families.
//
//	router bgp <asn>
//	 ...
//	exit
//	!
func genDefaultVRFSection(asn uint32, neighbors []string, selected *selectedNetworks) string {
	if asn == 0 || len(neighbors) == 0 {
		return ""
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "router bgp %d\n", asn)

	neighbors4, neighbors6 := splitNeighborsByIPFamily(neighbors)
	buf.WriteString(genUnicastSection(neighbors4, neighbors6))
	buf.WriteString(genDefaultVRFEVPNSection(neighbors, selected))

	buf.WriteString("exit\n!\n")

	return buf.String()
}

// genNonDefaultVRFSection generates a non-default VRF router's unicast and EVPN address-families.
//
//	router bgp <asn> vrf red
//	 ...
//	exit
//	!
func genNonDefaultVRFSection(vrf string, asn uint32, neighbors []string, cfg *ipVRFConfig) string {
	if vrf == "" || asn == 0 {
		return ""
	}
	if len(neighbors) == 0 && cfg == nil {
		return ""
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, vrf)

	neighbors4, neighbors6 := splitNeighborsByIPFamily(neighbors)
	buf.WriteString(genUnicastSection(neighbors4, neighbors6))
	buf.WriteString(genNonDefaultVRFEVPNSection(cfg))

	buf.WriteString("exit\n!\n")

	return buf.String()
}

// splitNeighborsByIPFamily splits neighbor identifiers by IP family. Unnumbered
// neighbors are identified by interface name rather than by address: FRR
// establishes their session over an IPv4 address derived from a /30 or /31 on
// the interface or, failing that, over the peer's IPv6 link-local address, and
// in either case the session can carry prefixes of both families (RFC 8950),
// so they are included in both results.
func splitNeighborsByIPFamily(neighbors []string) (neighbors4, neighbors6 []string) {
	for _, neighbor := range neighbors {
		switch {
		case utilnet.IsIPv6String(neighbor):
			neighbors6 = append(neighbors6, neighbor)
		case utilnet.IsIPv4String(neighbor):
			neighbors4 = append(neighbors4, neighbor)
		default:
			neighbors4 = append(neighbors4, neighbor)
			neighbors6 = append(neighbors6, neighbor)
		}
	}
	return
}

// genUnicastSection generates unicast address-family sections for IPv4 and/or IPv6 neighbors.
//
//	address-family ipv4 unicast
//	 neighbor <ip> allowas-in origin
//	exit-address-family
//	address-family ipv6 unicast
//	 neighbor <ip> allowas-in origin
//	exit-address-family
func genUnicastSection(neighbors4, neighbors6 []string) string {
	var buf strings.Builder

	if len(neighbors4) > 0 {
		buf.WriteString(" address-family ipv4 unicast\n")
		for _, neighbor := range neighbors4 {
			fmt.Fprintf(&buf, "  neighbor %s allowas-in origin\n", neighbor)
		}
		buf.WriteString(" exit-address-family\n")
	}
	if len(neighbors6) > 0 {
		buf.WriteString(" address-family ipv6 unicast\n")
		for _, neighbor := range neighbors6 {
			fmt.Fprintf(&buf, "  neighbor %s allowas-in origin\n", neighbor)
		}
		buf.WriteString(" exit-address-family\n")
	}
	return buf.String()
}

// genDefaultVRFEVPNSection generates the l2vpn evpn address-family for the default VRF.
//
//	address-family l2vpn evpn
//	 neighbor <ip> activate
//	 neighbor <ip> allowas-in origin
//	 advertise-all-vni
//	 vni <id>                           <- (Section only added when MAC-VRF RT is set)
//	  route-target import <rt>
//	  route-target export <rt>
//	 exit-vni
//	exit-address-family
func genDefaultVRFEVPNSection(neighbors []string, selected *selectedNetworks) string {
	hasEVPN := len(selected.ipVRFConfigs) > 0 || len(selected.macVRFConfigs) > 0
	if !hasEVPN {
		return ""
	}

	var buf strings.Builder

	buf.WriteString(" address-family l2vpn evpn\n")

	for _, neighbor := range neighbors {
		fmt.Fprintf(&buf, "  neighbor %s activate\n", neighbor)
		// Needed for eBGP peers sharing our ASN; no-op for iBGP. Applied
		// unconditionally because the peer type may be unknown (e.g.
		// `remote-as auto`, not yet supported by frr-k8s but anticipated).
		fmt.Fprintf(&buf, "  neighbor %s allowas-in origin\n", neighbor)
	}
	buf.WriteString("  advertise-all-vni\n")

	for _, cfg := range selected.macVRFConfigs {
		if cfg.RouteTarget == "" {
			continue
		}
		fmt.Fprintf(&buf, "  vni %d\n", cfg.VNI)
		fmt.Fprintf(&buf, "   route-target import %s\n", cfg.RouteTarget)
		fmt.Fprintf(&buf, "   route-target export %s\n", cfg.RouteTarget)
		buf.WriteString("  exit-vni\n")
	}

	buf.WriteString(" exit-address-family\n")

	return buf.String()
}

// genNonDefaultVRFEVPNSection generates the l2vpn evpn address-family for a non-default VRF.
//
//	address-family l2vpn evpn
//	 advertise ipv4 unicast
//	 advertise ipv6 unicast
//	 route-target import <rt>
//	 route-target export <rt>
//	exit-address-family
func genNonDefaultVRFEVPNSection(cfg *ipVRFConfig) string {
	if cfg == nil {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(" address-family l2vpn evpn\n")

	if cfg.HasIPv4 {
		buf.WriteString("  advertise ipv4 unicast\n")
	}
	if cfg.HasIPv6 {
		buf.WriteString("  advertise ipv6 unicast\n")
	}
	if cfg.RouteTarget != "" {
		fmt.Fprintf(&buf, "  route-target import %s\n", cfg.RouteTarget)
		fmt.Fprintf(&buf, "  route-target export %s\n", cfg.RouteTarget)
	}

	buf.WriteString(" exit-address-family\n")

	return buf.String()
}

// genCNCParentVRFSection generates the FRR stanza for an EVPN CNC parent VRF.
// The parent VRF is the L3 routing hub that imports routes from all child VRFs
// and re-advertises them via EVPN Type-5 routes.
//
// Generated config structure:
//
//	vrf <parentVRFName>
//	 vni <l3VNI>
//	exit-vrf
//	!
//	router bgp <asn> vrf <parentVRFName>
//	 address-family ipv4 unicast
//	  import vrf <child1>           <- one per child, sorted
//	  import vrf <child2>
//	 exit-address-family
//	 address-family l2vpn evpn
//	  advertise ipv4 unicast
//	  advertise ipv6 unicast
//	 exit-address-family
//	exit
//	!
func genCNCParentVRFSection(parentVRFName string, asn uint32, l3VNI int32, childVRFNames []string) string {
	if asn == 0 || l3VNI == 0 {
		return ""
	}

	var buf strings.Builder

	// VRF-to-VNI mapping for the parent VRF.
	fmt.Fprintf(&buf, "vrf %s\n vni %d\nexit-vrf\n!\n", parentVRFName, l3VNI)

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, parentVRFName)

	// ipv4 unicast section: import vrf directives for each child VRF.
	// Sorted for deterministic config generation.
	sorted := slices.Sorted(slices.Values(childVRFNames))
	if len(sorted) > 0 {
		buf.WriteString(" address-family ipv4 unicast\n")
		for _, child := range sorted {
			fmt.Fprintf(&buf, "  import vrf %s\n", child)
		}
		buf.WriteString(" exit-address-family\n")
	}

	// l2vpn evpn section: re-advertise imported routes as EVPN Type-5.
	buf.WriteString(" address-family l2vpn evpn\n")
	buf.WriteString("  advertise ipv4 unicast\n")
	buf.WriteString("  advertise ipv6 unicast\n")
	buf.WriteString(" exit-address-family\n")

	buf.WriteString("exit\n!\n")

	return buf.String()
}

// genCNCMACVRFChildSection generates the raw FRR stanza for a MAC-VRF-only child
// connected to a CNC parent VRF. MAC-VRF-only children have no dedicated IP-VRF VNI,
// so the CNC parent VRF acts as their shared IP-VRF. This stanza:
//   - redistributes connected routes into BGP so the parent can import and type-5-advertise them
//   - imports routes from the parent VRF for cross-network reachability
//
// Generated config structure:
//
//	router bgp <asn> vrf <childVRFName>
//	 address-family ipv4 unicast
//	  redistribute connected
//	  import vrf <parentVRFName>
//	 exit-address-family
//	 address-family ipv6 unicast
//	  redistribute connected
//	  import vrf <parentVRFName>
//	 exit-address-family
//	 address-family l2vpn evpn
//	  advertise ipv4 unicast
//	  advertise ipv6 unicast
//	 exit-address-family
//	exit
//	!
func genCNCMACVRFChildSection(childVRFName, parentVRFName string, asn uint32) string {
	if asn == 0 {
		return ""
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, childVRFName)

	buf.WriteString(" address-family ipv4 unicast\n")
	buf.WriteString("  redistribute connected\n")
	fmt.Fprintf(&buf, "  import vrf %s\n", parentVRFName)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString(" address-family ipv6 unicast\n")
	buf.WriteString("  redistribute connected\n")
	fmt.Fprintf(&buf, "  import vrf %s\n", parentVRFName)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString(" address-family l2vpn evpn\n")
	buf.WriteString("  advertise ipv4 unicast\n")
	buf.WriteString("  advertise ipv6 unicast\n")
	buf.WriteString(" exit-address-family\n")

	buf.WriteString("exit\n!\n")

	return buf.String()
}

// genCNCIPVRFChildImportSection generates the raw FRR stanza needed for an IP-VRF
// child connected to a CNC parent VRF. It adds a route-map on "advertise ipv4 unicast"
// to prevent routes imported from the parent VRF from being re-advertised under the
// child's own route-target — there is no structured FRRConfiguration equivalent for this.
//
//	ip prefix-list CNC-<CHILD>-PREFIXES seq 10 permit <subnet1>
//	ip prefix-list CNC-<CHILD>-PREFIXES seq 20 permit <subnet2>
//	!
//	route-map CNC-<CHILD>-ADVERTISE permit 10
//	 match ip address prefix-list CNC-<CHILD>-PREFIXES
//	!
//	router bgp <asn> vrf <childVRFName>
//	 address-family ipv4 unicast
//	  import vrf <parentVRFName>
//	 exit-address-family
//	 address-family l2vpn evpn
//	  advertise ipv4 unicast route-map CNC-<CHILD>-ADVERTISE
//	 exit-address-family
//	exit
//	!
func genCNCIPVRFChildImportSection(childVRFName, parentVRFName string, asn uint32, childSubnets []*net.IPNet) string {
	if asn == 0 {
		return ""
	}

	// childVRFName comes from GetNetworkVRFName which guarantees ≤15 chars,
	// so "CNC-" + upper(childVRFName) + "-PREFIXES/ADVERTISE" fits within FRR's 63-char limit.
	base := strings.ToUpper(childVRFName)
	prefixList := "CNC-" + base + "-PREFIXES"
	routeMap := "CNC-" + base + "-ADVERTISE"

	var buf strings.Builder

	if len(childSubnets) > 0 {
		// Emit prefix-list and route-map to filter re-advertisement of imported routes.
		// Subnets sorted for deterministic output.
		sorted := make([]*net.IPNet, len(childSubnets))
		copy(sorted, childSubnets)
		slices.SortFunc(sorted, func(a, b *net.IPNet) int {
			return strings.Compare(a.String(), b.String())
		})
		for i, subnet := range sorted {
			fmt.Fprintf(&buf, "ip prefix-list %s seq %d permit %s\n", prefixList, (i+1)*10, subnet)
		}
		buf.WriteString("!\n")
		fmt.Fprintf(&buf, "route-map %s permit 10\n", routeMap)
		fmt.Fprintf(&buf, " match ip address prefix-list %s\n", prefixList)
		buf.WriteString("!\n")
	}

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, childVRFName)

	buf.WriteString(" address-family ipv4 unicast\n")
	fmt.Fprintf(&buf, "  import vrf %s\n", parentVRFName)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString(" address-family l2vpn evpn\n")
	fmt.Fprintf(&buf, "  advertise ipv4 unicast route-map %s\n", routeMap)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString("exit\n!\n")

	return buf.String()
}
