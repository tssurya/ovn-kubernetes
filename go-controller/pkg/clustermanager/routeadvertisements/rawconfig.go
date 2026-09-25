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
	// re-advertisement of routes received from the parent VRF back into EVPN,
	// and to filter the parent-feedback-loop re-import of own subnets.
	IPVRFChildren []*cncIPVRFChild
	// MACVRFOnlyChildren are MAC-VRF-only children (no dedicated IP-VRF VNI).
	// Carries VRFName + Subnets so the import route-map can filter own subnets.
	MACVRFOnlyChildren []*cncIPVRFChild
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
	// routes from all child VRFs. All children need an import route-map that
	// filters their own subnets on "import vrf parent" to break the feedback
	// loop (parent re-imports child routes → re-advertises as type-5 → child
	// re-imports own routes → ECMP with type-2-derived /32 paths).
	// MAC-VRF-only children also need "redistribute connected" so their subnets
	// enter the BGP RIB for the parent to import and type-5-advertise.
	globalASN := vrfASNs[""]
	for _, cncCfg := range selected.cncParentVRFConfigs {
		var allChildSubnets []*net.IPNet
		for _, child := range cncCfg.IPVRFChildren {
			allChildSubnets = append(allChildSubnets, child.Subnets...)
		}
		for _, child := range cncCfg.MACVRFOnlyChildren {
			allChildSubnets = append(allChildSubnets, child.Subnets...)
		}
		buf.WriteString(genCNCParentVRFSection(cncCfg.ParentVRFName, globalASN, cncCfg.ParentVNI, cncCfg.ChildVRFNames, allChildSubnets))
		for _, child := range cncCfg.IPVRFChildren {
			buf.WriteString(genCNCIPVRFChildImportSection(child.VRFName, cncCfg.ParentVRFName, globalASN, child.Subnets))
		}
		for _, child := range cncCfg.MACVRFOnlyChildren {
			buf.WriteString(genCNCMACVRFChildSection(child.VRFName, cncCfg.ParentVRFName, globalASN, child.Subnets))
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
// allChildSubnets is the union of all connected children's subnets. When
// non-empty, a route-map is prepended to the parent's "import vrf" directives
// so the parent only absorbs CUDN subnets from children — not any external
// routes a child may have learned — preventing cross-CUDN external route leakage.
//
// Generated config structure (when allChildSubnets are provided):
//
//	ip prefix-list CNC-<PARENT>-PREFIXES seq 10 permit <v4subnet1>
//	ip prefix-list CNC-<PARENT>-PREFIXES seq 20 permit <v4subnet2>
//	!
//	route-map CNC-<PARENT>-IMPORT permit 10
//	 match ip address prefix-list CNC-<PARENT>-PREFIXES
//	route-map CNC-<PARENT>-IMPORT deny 20
//	!
//	vrf <parentVRFName>
//	 vni <l3VNI>
//	exit-vrf
//	!
//	router bgp <asn> vrf <parentVRFName>
//	 address-family ipv4 unicast
//	  import vrf route-map CNC-<PARENT>-IMPORT  <- only when subnets present
//	  import vrf <child1>                        <- one per child, sorted
//	  import vrf <child2>
//	 exit-address-family
//	 address-family l2vpn evpn
//	  advertise ipv4 unicast
//	  advertise ipv6 unicast
//	 exit-address-family
//	exit
//	!
func genCNCParentVRFSection(parentVRFName string, asn uint32, l3VNI int32, childVRFNames []string, allChildSubnets []*net.IPNet) string {
	if asn == 0 || l3VNI == 0 {
		return ""
	}

	// parentVRFName comes from GetCNCParentVRFName which guarantees ≤15 chars,
	// so "CNC-" + upper(parentVRFName) + "-PREFIXES/IMPORT" fits within FRR's 63-char limit.
	base := strings.ToUpper(parentVRFName)
	prefixList := "CNC-" + base + "-PREFIXES"
	importMap := "CNC-" + base + "-IMPORT"

	// Collect and sort IPv4 subnets from all children for the parent import filter.
	var v4Subnets []*net.IPNet
	for _, subnet := range allChildSubnets {
		if subnet.IP.To4() != nil {
			v4Subnets = append(v4Subnets, subnet)
		}
	}
	slices.SortFunc(v4Subnets, func(a, b *net.IPNet) int {
		return strings.Compare(a.String(), b.String())
	})

	var buf strings.Builder

	// Prefix-list and import route-map: permit only CUDN subnets, deny all else.
	// This prevents external routes learned by children from entering the parent
	// and being re-advertised cross-CUDN as EVPN Type-5.
	if len(v4Subnets) > 0 {
		for i, subnet := range v4Subnets {
			fmt.Fprintf(&buf, "ip prefix-list %s seq %d permit %s\n", prefixList, (i+1)*10, subnet)
		}
		buf.WriteString("!\n")
		fmt.Fprintf(&buf, "route-map %s permit 10\n", importMap)
		fmt.Fprintf(&buf, " match ip address prefix-list %s\n", prefixList)
		fmt.Fprintf(&buf, "route-map %s deny 20\n", importMap)
		buf.WriteString("!\n")
	}

	// VRF-to-VNI mapping for the parent VRF.
	fmt.Fprintf(&buf, "vrf %s\n vni %d\nexit-vrf\n!\n", parentVRFName, l3VNI)

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, parentVRFName)

	// ipv4 unicast section: import vrf directives for each child VRF.
	// Sorted for deterministic config generation.
	sorted := slices.Sorted(slices.Values(childVRFNames))
	if len(sorted) > 0 {
		buf.WriteString(" address-family ipv4 unicast\n")
		if len(v4Subnets) > 0 {
			fmt.Fprintf(&buf, "  import vrf route-map %s\n", importMap)
		}
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
//   - imports routes from the parent VRF for cross-network reachability, filtered by an
//     import route-map that blocks the child's own subnets from being re-installed via
//     the parent feedback loop (same ECMP issue as IP-VRF children)
//
// Generated config structure (when childSubnets are provided):
//
//	ip prefix-list CNC-<CHILD>-PREFIXES seq 10 permit <v4subnet1>
//	!
//	route-map CNC-<CHILD>-IMPORT deny 10
//	 match ip address prefix-list CNC-<CHILD>-PREFIXES
//	route-map CNC-<CHILD>-IMPORT permit 20
//	!
//	ipv6 prefix-list CNC-<CHILD>-V6PREFIXES seq 10 permit <v6subnet1>
//	!
//	route-map CNC-<CHILD>-V6IMPORT deny 10
//	 match ipv6 address prefix-list CNC-<CHILD>-V6PREFIXES
//	route-map CNC-<CHILD>-V6IMPORT permit 20
//	!
//	router bgp <asn> vrf <childVRFName>
//	 address-family ipv4 unicast
//	  redistribute connected
//	  import vrf route-map CNC-<CHILD>-IMPORT <parentVRFName>
//	  import vrf <parentVRFName>
//	 exit-address-family
//	 address-family ipv6 unicast
//	  redistribute connected
//	  import vrf route-map CNC-<CHILD>-V6IMPORT <parentVRFName>
//	  import vrf <parentVRFName>
//	 exit-address-family
//	 address-family l2vpn evpn
//	  advertise ipv4 unicast
//	  advertise ipv6 unicast
//	 exit-address-family
//	exit
//	!
func genCNCMACVRFChildSection(childVRFName, parentVRFName string, asn uint32, childSubnets []*net.IPNet) string {
	if asn == 0 {
		return ""
	}

	base := strings.ToUpper(childVRFName)
	v4PrefixList := "CNC-" + base + "-PREFIXES"
	v4ImportMap := "CNC-" + base + "-IMPORT"
	v6PrefixList := "CNC-" + base + "-V6PREFIXES"
	v6ImportMap := "CNC-" + base + "-V6IMPORT"

	// Separate subnets by IP family, sort for deterministic output.
	var v4Subnets, v6Subnets []*net.IPNet
	for _, subnet := range childSubnets {
		if subnet.IP.To4() != nil {
			v4Subnets = append(v4Subnets, subnet)
		} else {
			v6Subnets = append(v6Subnets, subnet)
		}
	}
	slices.SortFunc(v4Subnets, func(a, b *net.IPNet) int { return strings.Compare(a.String(), b.String()) })
	slices.SortFunc(v6Subnets, func(a, b *net.IPNet) int { return strings.Compare(a.String(), b.String()) })

	var buf strings.Builder

	if len(v4Subnets) > 0 {
		for i, subnet := range v4Subnets {
			fmt.Fprintf(&buf, "ip prefix-list %s seq %d permit %s\n", v4PrefixList, (i+1)*10, subnet)
		}
		buf.WriteString("!\n")
		fmt.Fprintf(&buf, "route-map %s deny 10\n", v4ImportMap)
		fmt.Fprintf(&buf, " match ip address prefix-list %s\n", v4PrefixList)
		fmt.Fprintf(&buf, "route-map %s permit 20\n", v4ImportMap)
		buf.WriteString("!\n")
	}
	if len(v6Subnets) > 0 {
		for i, subnet := range v6Subnets {
			fmt.Fprintf(&buf, "ipv6 prefix-list %s seq %d permit %s\n", v6PrefixList, (i+1)*10, subnet)
		}
		buf.WriteString("!\n")
		fmt.Fprintf(&buf, "route-map %s deny 10\n", v6ImportMap)
		fmt.Fprintf(&buf, " match ipv6 address prefix-list %s\n", v6PrefixList)
		fmt.Fprintf(&buf, "route-map %s permit 20\n", v6ImportMap)
		buf.WriteString("!\n")
	}

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, childVRFName)

	buf.WriteString(" address-family ipv4 unicast\n")
	buf.WriteString("  redistribute connected\n")
	if len(v4Subnets) > 0 {
		fmt.Fprintf(&buf, "  import vrf route-map %s\n", v4ImportMap)
	}
	fmt.Fprintf(&buf, "  import vrf %s\n", parentVRFName)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString(" address-family ipv6 unicast\n")
	buf.WriteString("  redistribute connected\n")
	if len(v6Subnets) > 0 {
		fmt.Fprintf(&buf, "  import vrf route-map %s\n", v6ImportMap)
	}
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
// child connected to a CNC parent VRF. It emits two route-maps:
//
//   - CNC-<CHILD>-ADVERTISE (permit 10): applied to "advertise ipv4 unicast" to prevent
//     routes imported from the parent VRF from being re-advertised under the child's own
//     route-target — there is no structured FRRConfiguration equivalent for this.
//
//   - CNC-<CHILD>-IMPORT (deny 10, permit 20): applied to "import vrf route-map" to
//     prevent the child's own subnets from being re-installed via the parent VRF feedback
//     loop (parent imports child routes, re-advertises as type-5, child would otherwise
//     re-import them creating ECMP against the type-2-derived /32 paths).
//
//	ip prefix-list CNC-<CHILD>-PREFIXES seq 10 permit <subnet1>
//	ip prefix-list CNC-<CHILD>-PREFIXES seq 20 permit <subnet2>
//	!
//	route-map CNC-<CHILD>-ADVERTISE permit 10
//	 match ip address prefix-list CNC-<CHILD>-PREFIXES
//	!
//	route-map CNC-<CHILD>-IMPORT deny 10
//	 match ip address prefix-list CNC-<CHILD>-PREFIXES
//	route-map CNC-<CHILD>-IMPORT permit 20
//	!
//	router bgp <asn> vrf <childVRFName>
//	 address-family ipv4 unicast
//	  import vrf route-map CNC-<CHILD>-IMPORT <parentVRFName>
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
	// so "CNC-" + upper(childVRFName) + "-PREFIXES/ADVERTISE/IMPORT" fits within FRR's 63-char limit.
	base := strings.ToUpper(childVRFName)
	prefixList := "CNC-" + base + "-PREFIXES"
	advertiseMap := "CNC-" + base + "-ADVERTISE"
	importMap := "CNC-" + base + "-IMPORT"

	var buf strings.Builder

	// Collect IPv4 subnets — the import route-map and advertise route-map both use
	// the same prefix-list. Sorted for deterministic output.
	var v4Subnets []*net.IPNet
	for _, subnet := range childSubnets {
		if subnet.IP.To4() != nil {
			v4Subnets = append(v4Subnets, subnet)
		}
	}
	slices.SortFunc(v4Subnets, func(a, b *net.IPNet) int {
		return strings.Compare(a.String(), b.String())
	})

	if len(v4Subnets) > 0 {
		for i, subnet := range v4Subnets {
			fmt.Fprintf(&buf, "ip prefix-list %s seq %d permit %s\n", prefixList, (i+1)*10, subnet)
		}
		buf.WriteString("!\n")
		// Advertise route-map: permit own subnets only (blocks re-advertisement of
		// routes imported from the parent back into EVPN under the child's RT).
		fmt.Fprintf(&buf, "route-map %s permit 10\n", advertiseMap)
		fmt.Fprintf(&buf, " match ip address prefix-list %s\n", prefixList)
		buf.WriteString("!\n")
		// Import route-map: deny own subnets (blocks feedback-loop re-import from
		// parent), permit everything else (sibling routes from parent).
		fmt.Fprintf(&buf, "route-map %s deny 10\n", importMap)
		fmt.Fprintf(&buf, " match ip address prefix-list %s\n", prefixList)
		fmt.Fprintf(&buf, "route-map %s permit 20\n", importMap)
		buf.WriteString("!\n")
	}

	fmt.Fprintf(&buf, "router bgp %d vrf %s\n", asn, childVRFName)

	buf.WriteString(" address-family ipv4 unicast\n")
	if len(v4Subnets) > 0 {
		fmt.Fprintf(&buf, "  import vrf route-map %s\n", importMap)
	}
	fmt.Fprintf(&buf, "  import vrf %s\n", parentVRFName)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString(" address-family l2vpn evpn\n")
	fmt.Fprintf(&buf, "  advertise ipv4 unicast route-map %s\n", advertiseMap)
	buf.WriteString(" exit-address-family\n")

	buf.WriteString("exit\n!\n")

	return buf.String()
}
