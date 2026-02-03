package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	udnv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	vtepv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"
	vtepclientset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/vtep/v1/apis/clientset/versioned"
	"github.com/ovn-org/ovn-kubernetes/test/e2e/deploymentconfig"
	"github.com/ovn-org/ovn-kubernetes/test/e2e/feature"
	"github.com/ovn-org/ovn-kubernetes/test/e2e/images"
	"github.com/ovn-org/ovn-kubernetes/test/e2e/infraprovider"
	infraapi "github.com/ovn-org/ovn-kubernetes/test/e2e/infraprovider/api"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	utilnet "k8s.io/utils/net"
)

// =============================================================================
// EVPN E2E Test Infrastructure
// =============================================================================
//
// This file contains utilities and tests for EVPN (Ethernet VPN) functionality
// in OVN-Kubernetes.
//
// =============================================================================
// Test Infrastructure Overview (PS: the values are just for illustration purposes)
// =============================================================================
//
// EVPN Test Infrastructure:
//
// +---------------------------------------+                      +------------------+
// | KIND Cluster                          |    KIND Primary      | External FRR     |
// |                                       |    Network           | (reused from BGP)|
// | +-------------+ +-------------+       |    (BGP/EVPN)        |                  |
// | | Node 1      | | Node 2      |       |    172.18.0.0/16     | eth0 (primary)   |
// | | - FRR-K8s   | | - FRR-K8s   |       | <----------------->  | br0 (EVPN)       |
// | | - OVN VTEP  | | - OVN VTEP  |       |                      | vxlan0 (VTEP)    |
// | +-------------+ +-------------+       |                      +------------------+
// |                                       |                             |
// | +-------------+ +----------------+    |         +------------------+--------------------+
// | | Node 3      | | Pod (on CUDN)  |    |         |   CUDN-network   | agnhost-ipvrf-net  |
// | | - FRR-K8s   | | 10.100.0.0/16  |    |         |   10.100.0.0/16  | 172.27.102.0/24    |
// | | - OVN VTEP  | +----------------+    |         +--------|---------+---------|---------+
// | +-------------+                       |                  |                   |
// +---------------------------------------+                  v                   v
//                                                   +------------------+--------------------+
//                                                   | agnhost-macvrf   | agnhost-ipvrf      |
//                                                   | 10.100.0.250     | 172.27.102.2       |
//                                                   | VNI: 10100       | VNI: 20102         |
//                                                   +------------------+--------------------+
//
// EVPN tests depend on the RouteAdvertisements/BGP infrastructure being set up
// during KIND cluster installation. EVPN tests REUSE the existing FRR container
// created at install time rather than creating a new one.
// We extend its configuration to support EVPN by adding:
//   - br0: Linux bridge with vlan_filtering for EVPN
//   - vxlan0: VXLAN device (VTEP) with vnifilter for SVD mode
//   - VRF and SVI configuration for IP-VRF tests
//   - Access ports for MAC-VRF tests
//   - BGP EVPN address-family configuration
//
// See OKEP: https://github.com/ovn-org/ovn-kubernetes/blob/master/docs/okeps/okep-5088-evpn.md for more details.

// =============================================================================
// EVPN Utilities
// =============================================================================

const (
	// externalFRRContainerName is the name of the external FRR container
	// created during KIND cluster setup with BGP enabled (./contrib/kind.sh -rae)
	externalFRRContainerName = "frr"
	// agnhostHTTPPort is the HTTP port for agnhost netexec
	agnhostHTTPPort = 8080
)

// setupEVPNBridgeOnExternalFRR creates the EVPN bridge (br0) and VXLAN device (vxlan0)
// on the external FRR container. This is the foundation for both MAC-VRF and IP-VRF tests.
//
// Creates:
//   - br0: Linux bridge with vlan_filtering enabled, vlan_default_pvid 0
//   - vxlan0: VXLAN device in SVD (Single VXLAN Device) mode with vnifilter
//
// The vxlan0 device is configured with:
//   - dstport 4789: Standard VXLAN port
//   - local <frrVTEPIPAddress>: Local VTEP IP (FRR's IP on KIND network) - in future when VTEP CR is implemented, this will be the VTEP IP
//   - nolearning: Disable MAC learning (controlled by BGP EVPN)
//   - external: Allow external FDB programming
//   - vnifilter: Enable per-VLAN VNI mapping (SVD mode)
//
// Cleanup is automatically registered via ictx.AddCleanUpFn().
func setupEVPNBridgeOnExternalFRR(ictx infraapi.Context, frrVTEPIPAddress string) error {
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

	// Create br0 bridge with VLAN filtering
	commands := [][]string{
		{"ip", "link", "add", "br0", "type", "bridge", "vlan_filtering", "1", "vlan_default_pvid", "0"},
		{"ip", "link", "set", "br0", "addrgenmode", "none"},
	}

	for _, cmd := range commands {
		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, cmd)
		if err != nil {
			return fmt.Errorf("failed to execute %v: %w", cmd, err)
		}
	}

	// Create vxlan0 in SVD mode
	vxlanCmd := []string{
		"ip", "link", "add", "vxlan0", "type", "vxlan",
		"dstport", "4789",
		"local", frrVTEPIPAddress,
		"nolearning",
		"external",
		"vnifilter",
	}
	_, err := infraprovider.Get().ExecExternalContainerCommand(frr, vxlanCmd)
	if err != nil {
		return fmt.Errorf("failed to create vxlan0: %w", err)
	}

	// Configure vxlan0
	commands = [][]string{
		{"ip", "link", "set", "vxlan0", "addrgenmode", "none"},
		{"ip", "link", "set", "vxlan0", "master", "br0"},
	}

	for _, cmd := range commands {
		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, cmd)
		if err != nil {
			return fmt.Errorf("failed to execute %v: %w", cmd, err)
		}
	}

	// Bring up interfaces
	commands = [][]string{
		{"ip", "link", "set", "br0", "up"},
		{"ip", "link", "set", "vxlan0", "up"},
	}

	for _, cmd := range commands {
		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, cmd)
		if err != nil {
			return fmt.Errorf("failed to execute %v: %w", cmd, err)
		}
	}

	// Configure vxlan0 bridge options for EVPN
	bridgeCmd := []string{
		"bridge", "link", "set", "dev", "vxlan0",
		"vlan_tunnel", "on",
		"neigh_suppress", "on",
		"learning", "off",
	}
	_, err = infraprovider.Get().ExecExternalContainerCommand(frr, bridgeCmd)
	if err != nil {
		return fmt.Errorf("failed to configure vxlan0 bridge options: %w", err)
	}

	// Register cleanup to remove br0 and vxlan0
	// Note: Deleting br0 also removes all associated bridge vlan and vni entries,
	// so explicit MAC-VRF cleanup is not needed.
	ictx.AddCleanUpFn(func() error {
		frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

		// Delete vxlan0 first (it's attached to br0)
		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, []string{"ip", "link", "del", "vxlan0"})
		if err != nil {
			return fmt.Errorf("failed to delete vxlan0: %w", err)
		}

		// Delete br0
		_, err = infraprovider.Get().ExecExternalContainerCommand(frr, []string{"ip", "link", "del", "br0"})
		if err != nil {
			return fmt.Errorf("failed to delete br0: %w", err)
		}

		framework.Logf("EVPN bridge cleanup complete on %s", externalFRRContainerName)
		return nil
	})

	framework.Logf("EVPN bridge setup complete on %s (br0 + vxlan0 with local IP %s)", externalFRRContainerName, frrVTEPIPAddress)
	return nil
}

// setupMACVRFOnExternalFRR configures MAC-VRF (Layer 2 EVPN) on the external FRR container.
// This adds the VLAN/VNI mapping to extend the L2 domain via EVPN Type-2/Type-3 routes.
//
// Requires: setupEVPNBridgeOnExternalFRR must be called first to create br0 and vxlan0.
//
// Parameters:
//   - vni: VXLAN Network Identifier (e.g., 10100)
//   - vid: VLAN ID for local bridging (e.g., 100)
func setupMACVRFOnExternalFRR(vni, vid int) error {
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}
	vidStr := fmt.Sprintf("%d", vid)
	vniStr := fmt.Sprintf("%d", vni)

	cmd := fmt.Sprintf(
		// Add VLAN to bridge
		"bridge vlan add dev br0 vid %s self && "+
			// Add VLAN to vxlan0
			"bridge vlan add dev vxlan0 vid %s && "+
			// Add VNI to vxlan0
			"bridge vni add dev vxlan0 vni %s && "+
			// Map VLAN to VNI (tunnel_info)
			"bridge vlan add dev vxlan0 vid %s tunnel_info id %s",
		vidStr, vidStr, vniStr, vidStr, vniStr)
	_, err := infraprovider.Get().ExecExternalContainerCommand(frr, []string{"sh", "-c", cmd})
	if err != nil {
		return fmt.Errorf("failed to setup MAC-VRF (VNI %d, VID %d): %w", vni, vid, err)
	}

	framework.Logf("MAC-VRF setup complete on %s (VNI %d, VID %d)", externalFRRContainerName, vni, vid)
	return nil
}

// setupIPVRFOnExternalFRR configures IP-VRF (Layer 3 EVPN) on the external FRR container.
// This creates a Linux VRF with SVI for L3 routing via EVPN Type-5 routes.
//
// Requires: setupEVPNBridgeOnExternalFRR must be called first to create br0 and vxlan0.
//
// Parameters:
//   - vrfName: Name of the Linux VRF (e.g., "vrf202")
//   - vni: VXLAN Network Identifier (e.g., 20102)
//   - vid: VLAN ID for the SVI (e.g., 202)
//
// Cleanup is automatically registered via ictx.AddCleanUpFn().
// Note: VLAN/VNI mappings are cleaned up when br0/vxlan0 are deleted.
func setupIPVRFOnExternalFRR(ictx infraapi.Context, vrfName string, vni, vid int) error {
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}
	vidStr := fmt.Sprintf("%d", vid)
	vniStr := fmt.Sprintf("%d", vni)

	// Create Linux VRF with routing table = VNI
	_, err := infraprovider.Get().ExecExternalContainerCommand(frr,
		[]string{"ip", "link", "add", vrfName, "type", "vrf", "table", vniStr})
	if err != nil {
		return fmt.Errorf("failed to create VRF %s: %w", vrfName, err)
	}

	// Bring up VRF
	_, err = infraprovider.Get().ExecExternalContainerCommand(frr,
		[]string{"ip", "link", "set", vrfName, "up"})
	if err != nil {
		return fmt.Errorf("failed to bring up VRF %s: %w", vrfName, err)
	}

	// Configure VLAN/VNI mapping (reuse MAC-VRF setup for this part)
	if err := setupMACVRFOnExternalFRR(vni, vid); err != nil {
		return fmt.Errorf("failed to configure VLAN/VNI mapping: %w", err)
	}

	// Create SVI (VLAN sub-interface on br0)
	sviName := fmt.Sprintf("br0.%d", vid)
	_, err = infraprovider.Get().ExecExternalContainerCommand(frr,
		[]string{"ip", "link", "add", sviName, "link", "br0", "type", "vlan", "id", vidStr})
	if err != nil {
		return fmt.Errorf("failed to create SVI %s: %w", sviName, err)
	}

	// Bind SVI to VRF
	_, err = infraprovider.Get().ExecExternalContainerCommand(frr,
		[]string{"ip", "link", "set", sviName, "master", vrfName})
	if err != nil {
		return fmt.Errorf("failed to bind SVI %s to VRF %s: %w", sviName, vrfName, err)
	}

	// Bring up SVI
	_, err = infraprovider.Get().ExecExternalContainerCommand(frr,
		[]string{"ip", "link", "set", sviName, "up"})
	if err != nil {
		return fmt.Errorf("failed to bring up SVI %s: %w", sviName, err)
	}

	// Register cleanup to remove SVI, Linux VRF, and FRR VRF definition
	ictx.AddCleanUpFn(func() error {
		frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

		// Delete SVI
		sviName := fmt.Sprintf("br0.%d", vid)
		_, err := infraprovider.Get().ExecExternalContainerCommand(frr,
			[]string{"ip", "link", "del", sviName})
		if err != nil {
			framework.Logf("Warning: failed to delete SVI %s: %v", sviName, err)
		}

		// Delete Linux VRF
		_, err = infraprovider.Get().ExecExternalContainerCommand(frr,
			[]string{"ip", "link", "del", vrfName})
		if err != nil {
			framework.Logf("Warning: failed to delete Linux VRF %s: %v", vrfName, err)
		}

		// Delete FRR VRF definition (now that Linux VRF is gone, FRR should allow this)
		_, err = infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(
			"configure terminal", fmt.Sprintf("no vrf %s", vrfName), "end",
		))
		if err != nil {
			framework.Logf("Warning: failed to delete FRR VRF definition %s: %v", vrfName, err)
		}

		framework.Logf("IP-VRF cleanup complete on %s (VRF %s, VID %d)", externalFRRContainerName, vrfName, vid)
		return nil
	})

	framework.Logf("IP-VRF setup complete on %s (VRF %s, VNI %d, VID %d)", externalFRRContainerName, vrfName, vni, vid)
	return nil
}

// vtyshCommand builds a shell command that invokes vtysh with single-quoted -c arguments.
// Using sh -c with single-quoted args ensures correct argument parsing regardless of
// how the infra provider executes the command (docker exec, podman exec, SSH, etc.).
func vtyshCommand(args ...string) []string {
	var parts []string
	for _, arg := range args {
		parts = append(parts, fmt.Sprintf("-c '%s'", arg))
	}
	return []string{"sh", "-c", "vtysh " + strings.Join(parts, " ")}
}

// setupEVPNBGPOnExternalFRR configures the global BGP settings for EVPN on the external FRR container.
// This enables the l2vpn evpn address-family, advertise-all-vni, and activates neighbors.
//
// This is required for ALL EVPN scenarios (MAC-VRF and IP-VRF).
// advertise-all-vni automatically handles MAC-VRF route advertisement.
//
// Parameters:
//   - asn: BGP Autonomous System Number (e.g., 64512)
//   - neighborIPs: List of cluster node IPs to peer with
//
// Cleanup is automatically registered via ictx.AddCleanUpFn().
func setupEVPNBGPOnExternalFRR(ictx infraapi.Context, asn int, neighborIPs []string) error {
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

	args := []string{"configure terminal", fmt.Sprintf("router bgp %d", asn), "address-family l2vpn evpn", "advertise-all-vni"}
	for _, ip := range neighborIPs {
		args = append(args, fmt.Sprintf("neighbor %s activate", ip))
	}
	args = append(args, "exit-address-family", "end")

	_, err := infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(args...))
	if err != nil {
		return fmt.Errorf("failed to configure EVPN BGP: %w", err)
	}

	// Register cleanup to remove advertise-all-vni and deactivate neighbors
	ictx.AddCleanUpFn(func() error {
		frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

		args := []string{"configure terminal", fmt.Sprintf("router bgp %d", asn), "address-family l2vpn evpn", "no advertise-all-vni"}
		for _, ip := range neighborIPs {
			args = append(args, fmt.Sprintf("no neighbor %s activate", ip))
		}
		args = append(args, "exit-address-family", "end")

		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(args...))
		if err != nil {
			return fmt.Errorf("failed to cleanup EVPN BGP: %w", err)
		}

		framework.Logf("EVPN BGP cleanup complete on %s", externalFRRContainerName)
		return nil
	})

	framework.Logf("EVPN BGP setup complete on %s (ASN %d, neighbors: %v)", externalFRRContainerName, asn, neighborIPs)
	return nil
}

// setupIPVRFBGPOnExternalFRR configures BGP for an IP-VRF on the external FRR container.
// This binds the VRF to a VNI and configures route-targets for Type-5 route exchange.
//
// Requires: setupEVPNBGPOnExternalFRR must be called first for global EVPN BGP config.
//
// Parameters:
//   - vrfName: Name of the Linux VRF (must match the VRF created by setupIPVRFOnExternalFRR)
//   - ipVRFBGPASN: BGP ASN for this VRF's BGP instance (e.g., 64600). Can differ from bgpASN
//     since each VRF has its own BGP instance. This enables eBGP for N-S traffic testing.
//     See: https://docs.frrouting.org/en/stable-10.2/bgp.html#multiple-autonomous-systems
//   - bgpASN: General BGP ASN (e.g., 64512) used for Route Distinguisher and Route Target.
//     Must match the cluster's frr-k8s ASN to ensure EVPN route import/export works correctly.
//   - vni: VXLAN Network Identifier for route-target (e.g., 20102)
//   - ipFamilies: IP families to configure (e.g., sets.New(utilnet.IPv4) or sets.New(utilnet.IPv4, utilnet.IPv6))
//   - subnets: Subnets to advertise via BGP (e.g., []string{"172.27.102.0/24"} or {"172.27.102.0/24", "fd00:102::/64"})
//
// Cleanup is automatically registered via ictx.AddCleanUpFn().
func setupIPVRFBGPOnExternalFRR(ictx infraapi.Context, vrfName string, ipVRFBGPASN, bgpASN, vni int, ipFamilies sets.Set[utilnet.IPFamily], subnets []string) error {
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}
	// Route-target uses bgpASN (must match cluster's frr-k8s ASN for route import/export)
	rt := fmt.Sprintf("%d:%d", bgpASN, vni)

	// Build vtysh args
	args := []string{
		"configure terminal",
		fmt.Sprintf("vrf %s", vrfName), fmt.Sprintf("vni %d", vni), "exit-vrf",
		fmt.Sprintf("router bgp %d vrf %s", ipVRFBGPASN, vrfName),
	}

	// Configure address-families with explicit network statements
	// (Using explicit 'network' statements instead of 'redistribute connected' to align
	// with how OVN-K advertises routes via RouteAdvertisements/FRRConfiguration Prefixes)
	if ipFamilies.Has(utilnet.IPv4) {
		args = append(args, "address-family ipv4 unicast")
		for _, subnet := range subnets {
			if !utilnet.IsIPv6CIDRString(subnet) {
				args = append(args, fmt.Sprintf("network %s", subnet))
			}
		}
		args = append(args, "exit-address-family")
	}
	if ipFamilies.Has(utilnet.IPv6) {
		args = append(args, "address-family ipv6 unicast")
		for _, subnet := range subnets {
			if utilnet.IsIPv6CIDRString(subnet) {
				args = append(args, fmt.Sprintf("network %s", subnet))
			}
		}
		args = append(args, "exit-address-family")
	}

	// l2vpn evpn - configure RD, RT, and advertise unicast routes
	args = append(args, "address-family l2vpn evpn", fmt.Sprintf("rd %s", rt), fmt.Sprintf("route-target import %s", rt), fmt.Sprintf("route-target export %s", rt))
	if ipFamilies.Has(utilnet.IPv4) {
		args = append(args, "advertise ipv4 unicast")
	}
	if ipFamilies.Has(utilnet.IPv6) {
		args = append(args, "advertise ipv6 unicast")
	}
	args = append(args, "exit-address-family", "end")

	_, err := infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(args...))
	if err != nil {
		return fmt.Errorf("failed to configure IP-VRF BGP for %s: %w", vrfName, err)
	}

	// Register cleanup to remove BGP VRF instance and VRF-VNI binding
	// NOTE: This cleanup may run after setupIPVRFOnExternalFRR cleanup has already deleted
	// the Linux VRF device, which triggers FRR to auto-cleanup the VRF config. In that case,
	// these commands may fail - that's OK, we just log warnings and won't retry that.
	ictx.AddCleanUpFn(func() error {
		frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

		_, err := infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(
			"configure terminal", fmt.Sprintf("vrf %s", vrfName), fmt.Sprintf("no vni %d", vni), "exit-vrf", "end",
		))
		if err != nil {
			framework.Logf("Warning: failed to remove VNI binding (may already be cleaned up): %v", err)
		}

		_, err = infraprovider.Get().ExecExternalContainerCommand(frr, vtyshCommand(
			"configure terminal", fmt.Sprintf("no router bgp %d vrf %s", ipVRFBGPASN, vrfName), "end",
		))
		if err != nil {
			framework.Logf("Warning: failed to remove BGP VRF (may already be cleaned up): %v", err)
		}

		// NOTE: We intentionally do NOT run "no vrf" here.
		// FRR's VRF definition is tied to the Linux VRF and will auto-cleanup
		// when the Linux VRF is deleted by setupIPVRFOnExternalFRR's cleanup.
		// Trying to delete the FRR VRF while the Linux VRF still has interfaces
		// attached causes "Only inactive VRFs can be deleted" errors.

		framework.Logf("IP-VRF BGP cleanup complete on %s (VRF %s)", externalFRRContainerName, vrfName)
		return nil
	})

	framework.Logf("IP-VRF BGP setup complete on %s (VRF %s, ipVRFBGPASN %d, bgpASN %d, VNI %d, RT %s, families %v)", externalFRRContainerName, vrfName, ipVRFBGPASN, bgpASN, vni, rt, ipFamilies.UnsortedList())
	return nil
}

// =============================================================================
// EVPN Agnhost Utilities
// =============================================================================

// evpnAgnhostInfo holds the discovered network information for an EVPN agnhost container.
type evpnAgnhostInfo struct {
	agnhostIPs       []string
	agnhostInterface string
	frrIPs           []string
	frrInterface     string
}

// createEVPNAgnhost creates a Docker network with the given subnets, creates an agnhost
// container on it, attaches FRR to it, and discovers the assigned IPs and interface names.
//
// This is the shared foundation for both MAC-VRF and IP-VRF agnhost setups.
// The caller is responsible for configuring FRR's interface (e.g., adding it to br0
// as an access port for MAC-VRF, or putting it in a VRF for IP-VRF).
//
// Parameters:
//   - networkName: Name for the Docker network (e.g., "macvrf-net-100", "ipvrf-net-202")
//   - containerName: Name for the agnhost container (e.g., "agnhost-macvrf-100")
//   - ipFamilies: Cluster IP family support, used to filter discovered IPs
//   - subnets: Subnets for the Docker network (e.g., "10.100.0.0/16" for IPv4, or both for dual-stack)
func createEVPNAgnhost(ictx infraapi.Context, networkName, containerName string, ipFamilies sets.Set[utilnet.IPFamily], subnets ...string) (*evpnAgnhostInfo, error) {
	// Step 1: Create Docker network with specific subnet(s)
	network, err := ictx.CreateNetwork(networkName, subnets...)
	if err != nil {
		return nil, fmt.Errorf("failed to create network %s: %w", networkName, err)
	}

	// Step 2: Create agnhost container on that network
	agnhostContainer := infraapi.ExternalContainer{
		Name:        containerName,
		Image:       images.AgnHost(),
		Network:     network,
		CmdArgs:     []string{"netexec", fmt.Sprintf("--http-port=%d", agnhostHTTPPort)},
		RuntimeArgs: []string{"--cap-add=NET_ADMIN"},
	}
	_, err = ictx.CreateExternalContainer(agnhostContainer)
	if err != nil {
		return nil, fmt.Errorf("failed to create agnhost container %s: %w", containerName, err)
	}

	// Step 3: Connect FRR to the network
	_, err = ictx.AttachNetwork(network, externalFRRContainerName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect FRR to network %s: %w", networkName, err)
	}

	// Step 4: Discover assigned IPs and interface names
	agnhostNetInf, err := infraprovider.Get().GetExternalContainerNetworkInterface(
		infraapi.ExternalContainer{Name: containerName}, network)
	if err != nil {
		return nil, fmt.Errorf("failed to get agnhost network interface: %w", err)
	}

	frrNetInf, err := infraprovider.Get().GetExternalContainerNetworkInterface(
		infraapi.ExternalContainer{Name: externalFRRContainerName}, network)
	if err != nil {
		return nil, fmt.Errorf("failed to get FRR network interface: %w", err)
	}

	frrInterface := frrNetInf.InfName
	if frrInterface == "" {
		return nil, fmt.Errorf("FRR interface name not found for network %s", networkName)
	}

	agnhostInterface := agnhostNetInf.InfName
	if agnhostInterface == "" {
		return nil, fmt.Errorf("agnhost interface name not found for network %s", networkName)
	}

	// Collect IPs only for cluster-supported address families.
	// Docker may assign IPs for families we didn't request (e.g., default IPv4 on IPv6-only networks),
	// so we filter based on what the cluster actually supports.
	var agnhostIPs, frrIPs []string
	if ipFamilies.Has(utilnet.IPv4) {
		if agnhostNetInf.IPv4 != "" {
			agnhostIPs = append(agnhostIPs, agnhostNetInf.IPv4)
		}
		if frrNetInf.IPv4 != "" {
			frrIPs = append(frrIPs, frrNetInf.IPv4)
		}
	}
	if ipFamilies.Has(utilnet.IPv6) {
		if agnhostNetInf.IPv6 != "" {
			agnhostIPs = append(agnhostIPs, agnhostNetInf.IPv6)
		}
		if frrNetInf.IPv6 != "" {
			frrIPs = append(frrIPs, frrNetInf.IPv6)
		}
	}

	framework.Logf("EVPN agnhost created: %s (agnhost IPs: %v, FRR IPs: %v, interface: %s, FRR interface: %s)", containerName, agnhostIPs, frrIPs, agnhostInterface, frrInterface)
	return &evpnAgnhostInfo{
		agnhostIPs:       agnhostIPs,
		agnhostInterface: agnhostInterface,
		frrIPs:           frrIPs,
		frrInterface:     frrInterface,
	}, nil
}

// =============================================================================
// MAC-VRF Agnhost Utilities
// =============================================================================

// secondToLastIP returns the second-to-last usable IP in the given subnet.
// Using the high end of the range avoids collisions with both OVN IPAM
// (which allocates from lower end onwards) and Docker IPAM (which allocates from lower end onwards).
// This assumes OVN-K CUDN IPAM won't allocate IPs from the top of the subnet range
// for pods in these e2e tests.
// Example: "10.100.0.0/24" -> 10.100.0.253, "fd00:100::/64" -> fd00:100::ffff:ffff:ffff:fffe
func secondToLastIP(ipNet *net.IPNet) net.IP {
	// Compute broadcast: network OR inverted mask
	broadcast := make(net.IP, len(ipNet.IP))
	for i := range ipNet.IP {
		broadcast[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}
	// Subtract 2 from broadcast to get second-to-last usable IP
	result := make(net.IP, len(broadcast))
	copy(result, broadcast)
	borrow := byte(2)
	for i := len(result) - 1; i >= 0 && borrow > 0; i-- {
		diff := int(result[i]) - int(borrow)
		if diff < 0 {
			result[i] = byte(diff + 256)
			borrow = 1
		} else {
			result[i] = byte(diff)
			borrow = 0
		}
	}
	return result
}

// setupMACVRFAgnhost creates an agnhost container connected to the EVPN bridge
// for MAC-VRF (Layer 2) connectivity testing.
//
// This function:
//  1. Creates a Docker network with the CUDN subnet and an agnhost on it
//  2. Connects FRR to the network (Docker creates a veth pair automatically)
//  3. Replaces Docker-assigned IPs on agnhost with the second-to-last IP of the subnet
//     to avoid collisions with OVN IPAM and Docker IPAM
//  4. Moves FRR's interface to br0 as an access port for the MAC-VRF VLAN
//
// Requires: setupEVPNBridgeOnExternalFRR and setupMACVRFOnExternalFRR must be called first.
//
// The agnhost will be on the same L2 segment as pods on the CUDN, allowing
// direct Layer 2 communication via EVPN Type-2/Type-3 routes.
//
// Parameters:
//   - vid: VLAN ID for the access port on br0 (e.g., 100)
//   - ipFamilies: Cluster IP family support (e.g., sets.New(utilnet.IPv4, utilnet.IPv6))
//   - subnets: Subnets for the Docker network matching the CUDN (e.g., "10.100.0.0/16")
//
// Returns:
//   - Agnhost's IP addresses without prefix (e.g., ["10.100.0.253"] for a /24 subnet)
func setupMACVRFAgnhost(ictx infraapi.Context, vid int, ipFamilies sets.Set[utilnet.IPFamily], subnets ...string) ([]string, error) {
	networkName := fmt.Sprintf("macvrf-net-%d", vid)
	containerName := fmt.Sprintf("agnhost-macvrf-%d", vid)

	info, err := createEVPNAgnhost(ictx, networkName, containerName, ipFamilies, subnets...)
	if err != nil {
		return nil, err
	}

	// Replace Docker-assigned IPs with the second-to-last IP of the subnet.
	// Docker and OVN IPAM both allocate from the low end of the subnet,
	// so using the high end avoids collisions on the shared L2 segment.
	// We use targeted "ip addr del" (not flush) to preserve IPv6 link-local addresses
	// needed for NDP on the L2 segment.
	agnhost := infraapi.ExternalContainer{Name: containerName}

	var ipCmds []string
	var agnhostIPs []string
	for _, subnet := range subnets {
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return nil, fmt.Errorf("failed to parse subnet %s: %w", subnet, err)
		}
		ones, _ := ipNet.Mask.Size()
		// Delete the Docker-assigned IP that falls within this subnet
		for _, dockerIP := range info.agnhostIPs {
			if ipNet.Contains(net.ParseIP(dockerIP)) {
				ipCmds = append(ipCmds, fmt.Sprintf("ip addr del %s/%d dev %s", dockerIP, ones, info.agnhostInterface))
				break
			}
		}
		// Add the second-to-last usable IP of the subnet
		newIP := secondToLastIP(ipNet)
		ipCmds = append(ipCmds, fmt.Sprintf("ip addr add %s/%d dev %s", newIP, ones, info.agnhostInterface))
		agnhostIPs = append(agnhostIPs, newIP.String())
	}

	if len(ipCmds) > 0 {
		combined := strings.Join(ipCmds, " && ")
		if _, err := infraprovider.Get().ExecExternalContainerCommand(agnhost, []string{"sh", "-c", combined}); err != nil {
			return nil, fmt.Errorf("failed to replace Docker IPs on agnhost: %w", err)
		}
	}

	// Move FRR's interface to br0 and configure as access port
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}
	vidStr := fmt.Sprintf("%d", vid)
	frrCmd := fmt.Sprintf("ip link set %s master br0 && bridge vlan add dev %s vid %s pvid untagged",
		info.frrInterface, info.frrInterface, vidStr)
	if _, err = infraprovider.Get().ExecExternalContainerCommand(frr, []string{"sh", "-c", frrCmd}); err != nil {
		return nil, fmt.Errorf("failed to configure %s as br0 access port for VID %s: %w", info.frrInterface, vidStr, err)
	}

	// Cleanup is handled automatically:
	// - ictx.CreateNetwork() registers network deletion
	// - ictx.CreateExternalContainer() registers container deletion
	// - ictx.AttachNetwork() registers network detachment

	framework.Logf("MAC-VRF agnhost setup complete: %s (IPs: %v, VID: %d, FRR interface: %s)", containerName, agnhostIPs, vid, info.frrInterface)
	return agnhostIPs, nil
}

// =============================================================================
// IP-VRF Agnhost Utilities
// =============================================================================

// setupIPVRFAgnhost creates an agnhost container connected to the external FRR's VRF
// for IP-VRF (Layer 3) connectivity testing.
//
// This function:
//  1. Creates a Docker network with the specified subnet
//  2. Creates an agnhost container on that network
//  3. Connects FRR to the network
//  4. Discovers assigned IPs (Docker assigns them from subnet)
//  5. Puts FRR's interface for that network into the VRF
//  6. Sets agnhost's default route via FRR
//
// Requires: setupIPVRFOnExternalFRR must be called first to create the VRF.
//
// The agnhost will be on a separate routed subnet, reachable via EVPN Type-5 routes.
//
// Names are derived from VID to support multiple IP-VRF agnhosts:
//   - Network: ipvrf-net-<vid> (e.g., ipvrf-net-202)
//   - Container: agnhost-ipvrf-<vid> (e.g., agnhost-ipvrf-202)
//
// Parameters:
//   - vid: VLAN ID used to derive names (should match the VRF's VID)
//   - vrfName: Name of the VRF to put FRR's interface in (must match setupIPVRFOnExternalFRR)
//   - ipFamilies: Cluster IP family support (e.g., sets.New(utilnet.IPv4, utilnet.IPv6))
//   - subnets: Subnets for the Docker network (e.g., "172.27.102.0/24" for IPv4, or both for dual-stack)
//
// Returns:
//   - Agnhost's IP addresses (IPv4 and/or IPv6 depending on cluster IP family support)
func setupIPVRFAgnhost(ictx infraapi.Context, vid int, vrfName string, ipFamilies sets.Set[utilnet.IPFamily], subnets ...string) ([]string, error) {
	networkName := fmt.Sprintf("ipvrf-net-%d", vid)
	containerName := fmt.Sprintf("agnhost-ipvrf-%d", vid)

	info, err := createEVPNAgnhost(ictx, networkName, containerName, ipFamilies, subnets...)
	if err != nil {
		return nil, err
	}

	// Put FRR's interface in the VRF
	// NOTE: keep_addr_on_down=1 is set at FRR container startup (in deploy_frr_external_container)
	// to preserve IPv6 addresses during VRF assignment. See https://github.com/FRRouting/frr/issues/1666
	frr := infraapi.ExternalContainer{Name: externalFRRContainerName}

	frrCmd := fmt.Sprintf("ip link set %s master %s && ip link set %s up", info.frrInterface, vrfName, info.frrInterface)
	if _, err = infraprovider.Get().ExecExternalContainerCommand(frr, []string{"sh", "-c", frrCmd}); err != nil {
		return nil, fmt.Errorf("failed to assign %s to VRF %s: %w", info.frrInterface, vrfName, err)
	}

	// Set agnhost's default routes via FRR (for each address family)
	agnhost := infraapi.ExternalContainer{Name: containerName}

	var routeCmds []string
	for _, gwIP := range info.frrIPs {
		family := ""
		if utilnet.IsIPv6String(gwIP) {
			family = "-6 "
		}
		routeCmds = append(routeCmds, fmt.Sprintf("ip %sroute replace default via %s dev %s", family, gwIP, info.agnhostInterface))
	}
	if len(routeCmds) > 0 {
		combined := strings.Join(routeCmds, " && ")
		if _, err = infraprovider.Get().ExecExternalContainerCommand(agnhost, []string{"sh", "-c", combined}); err != nil {
			return nil, fmt.Errorf("failed to set default routes on agnhost: %w", err)
		}
	}

	// Cleanup is handled automatically:
	// - ictx.CreateNetwork() registers network deletion
	// - ictx.CreateExternalContainer() registers container deletion
	// - ictx.AttachNetwork() registers network detachment

	framework.Logf("IP-VRF agnhost setup complete: %s (IPs: %v, network: %s, VRF: %s)", containerName, info.agnhostIPs, networkName, vrfName)
	return info.agnhostIPs, nil
}

// =============================================================================
// VTEP Utilities
// =============================================================================

// createVTEP creates a VTEP (VXLAN Tunnel Endpoint) custom resource for EVPN.
// The VTEP CR defines the IP range from which VTEP IPs are allocated for EVPN VXLAN tunnels.
//
// Parameters:
//   - f: Test framework (used to get client config)
//   - ictx: Infrastructure context for cleanup registration
//   - name: Name of the VTEP CR
//   - cidrs: CIDR ranges for VTEP IP allocation (supports dual-stack with 2 CIDRs)
//   - mode: VTEP mode - "Managed" (OVN-K allocates IPs) or "Unmanaged" (external provider)
func createVTEP(f *framework.Framework, ictx infraapi.Context, name string, cidrs []string, mode vtepv1.VTEPMode) error {
	client, err := vtepclientset.NewForConfig(f.ClientConfig())
	if err != nil {
		return fmt.Errorf("failed to create VTEP client: %w", err)
	}

	// Convert string CIDRs to vtepv1.CIDR type
	vtepCIDRs := make(vtepv1.DualStackCIDRs, len(cidrs))
	for i, cidr := range cidrs {
		vtepCIDRs[i] = vtepv1.CIDR(cidr)
	}

	vtep := &vtepv1.VTEP{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: vtepv1.VTEPSpec{
			CIDRs: vtepCIDRs,
			Mode:  mode,
		},
	}

	_, err = client.K8sV1().VTEPs().Create(context.Background(), vtep, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VTEP %s: %w", name, err)
	}

	// Register cleanup: delete VTEP and wait until it's fully removed
	ictx.AddCleanUpFn(func() error {
		err := client.K8sV1().VTEPs().Delete(context.Background(), name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := client.K8sV1().VTEPs().Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
	})

	// TODO: Add status check once VTEP controller implements status conditions
	framework.Logf("VTEP created: %s (CIDRs: %v, Mode: %s)", name, cidrs, mode)
	return nil
}

// =============================================================================
// EVPN VID Utilities
// =============================================================================

// randomVID generates a random VLAN ID in the valid range (2-4094).
// VIDs 0, 1, and 4095 are reserved and should not be used.
func randomVID() int {
	return rand.Intn(4093) + 2 // 2-4094
}

// randomVNI generates a random VXLAN Network Identifier in the valid 24-bit range (1-16777215).
func randomVNI() int32 {
	return int32(rand.Intn(16777215) + 1)
}

// =============================================================================
// FRRConfiguration Utilities
// =============================================================================

// createFRRConfiguration creates an FRRConfiguration CR for BGP peering with the external FRR.
// This is used by RouteAdvertisements to determine which neighbors to advertise routes to.
//
// For EVPN L3 IP-VRF, we don't need toReceive because:
// - External routes come via EVPN Type-5 and are imported via route-target matching in the VRF
// - The FRRConfiguration just provides the BGP neighbor definition and label for RA selector
//
// Parameters:
//   - ictx: Infrastructure context for cleanup registration
//   - name: Name of the FRRConfiguration CR
//   - namespace: Namespace for the FRRConfiguration (typically frr-k8s-system)
//   - asn: BGP Autonomous System Number (e.g., 64512)
//   - neighborIP: IP address of the external FRR to peer with
//   - labels: Labels to apply to the FRRConfiguration (used by RouteAdvertisements selector)
func createFRRConfiguration(ictx infraapi.Context,
	name, namespace string,
	asn int,
	neighborIP string,
	labels map[string]string) error {

	// Build labels string for YAML
	labelsYAML := ""
	for k, v := range labels {
		labelsYAML += fmt.Sprintf("    %s: %s\n", k, v)
	}

	// Generate FRRConfiguration YAML
	// No toReceive needed - EVPN routes come via l2vpn evpn address-family
	// and are imported via route-target matching in the VRF
	yaml := fmt.Sprintf(`apiVersion: frrk8s.metallb.io/v1beta1
kind: FRRConfiguration
metadata:
  name: %s
  namespace: %s
  labels:
%sspec:
  bgp:
    routers:
    - asn: %d
      neighbors:
      - address: %s
        asn: %d
        disableMP: true
`, name, namespace, labelsYAML, asn, neighborIP, asn)

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "frrconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yaml); err != nil {
		return fmt.Errorf("failed to write FRRConfiguration YAML: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Apply via kubectl
	_, err = e2ekubectl.RunKubectl(namespace, "create", "-f", tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to create FRRConfiguration: %w", err)
	}

	// Register cleanup
	ictx.AddCleanUpFn(func() error {
		_, err := e2ekubectl.RunKubectl(namespace, "delete", "frrconfiguration", name, "--ignore-not-found")
		if err != nil {
			return fmt.Errorf("failed to delete FRRConfiguration %s: %w", name, err)
		}
		framework.Logf("FRRConfiguration deleted: %s", name)
		return nil
	})

	framework.Logf("FRRConfiguration created: %s (neighbor: %s)", name, neighborIP)
	return nil
}

// =============================================================================
// REVERT ME: Temporary Cluster-Side EVPN Setup
// =============================================================================
// This section provides a temporary workaround to enable EVPN E2E tests before
// OVN-Kubernetes natively implements EVPN. It downloads and executes a setup
// script from a remote GitHub repository.
//
// The script performs:
//   - Step 7: Setup EVPN Bridge on cluster nodes (br0/vxlan0)
//   - Step 8: Configure MAC-VRF + IP-VRF (OVS port, OVN LSP, VLAN/VNI, SVI)
//   - Step 9: Configure frr-k8s for EVPN BGP (vtysh commands)
//
// Remove this entire section once OVN-K EVPN implementation is complete.
// =============================================================================

const (
	// clusterEVPNSetupScriptURL is the URL to the cluster-side EVPN setup script.
	// REVERT ME: Remove this once OVN-K implements EVPN natively.
	clusterEVPNSetupScriptURL = "https://raw.githubusercontent.com/tssurya/evpn-scripts-on-ovnk-kind-with-existing-bgp-setup/main/evpn-cluster-setup.sh"
)

// runClusterEVPNSetupScript executes the cluster-side EVPN setup script from remote URL.
// The script discovers and configures all cluster nodes automatically using kubectl.
// This is a temporary workaround until OVN-K implements EVPN natively.
//
// REVERT ME: Remove this function once OVN-K EVPN implementation is complete.
func runClusterEVPNSetupScript(ictx infraapi.Context,
	networkName, externalFRRIP string,
	bgpASN int,
	hasMACVRF bool, macVRFVNI, macVRFVID int,
	hasIPVRF bool, ipVRFVNI, ipVRFVID int,
	cudnSubnets []string) error {

	// Build environment variables - script handles node discovery via kubectl
	// All vars prefixed with EVPN_ to avoid conflicts with other env vars
	env := map[string]string{
		"EVPN_NETWORK_NAME":    networkName,
		"EVPN_EXTERNAL_FRR_IP": externalFRRIP,
		"EVPN_BGP_ASN":         fmt.Sprintf("%d", bgpASN),
		"EVPN_CUDN_SUBNETS":    strings.Join(cudnSubnets, ","),
		"EVPN_OVN_NAMESPACE":   deploymentconfig.Get().OVNKubernetesNamespace(),
		"EVPN_FRR_NAMESPACE":   deploymentconfig.Get().FRRK8sNamespace(),
	}
	if hasMACVRF {
		env["EVPN_MACVRF_VNI"] = fmt.Sprintf("%d", macVRFVNI)
		env["EVPN_MACVRF_VID"] = fmt.Sprintf("%d", macVRFVID)
	}
	if hasIPVRF {
		env["EVPN_IPVRF_VNI"] = fmt.Sprintf("%d", ipVRFVNI)
		env["EVPN_IPVRF_VID"] = fmt.Sprintf("%d", ipVRFVID)
	}

	// Register cleanup FIRST - ensures cleanup runs even if setup fails
	ictx.AddCleanUpFn(func() error {
		cleanupEnv := copyEnvMap(env)
		cleanupEnv["EVPN_CLEANUP"] = "true"

		framework.Logf("Running cluster EVPN cleanup script")
		if err := runRemoteScript(clusterEVPNSetupScriptURL, cleanupEnv); err != nil {
			// Log but don't fail - cleanup should be best-effort
			framework.Logf("WARNING: cleanup script had errors (may be expected): %v", err)
		}
		return nil
	})

	// Run setup script - it handles all nodes internally
	framework.Logf("Running cluster EVPN setup script")
	if err := runRemoteScript(clusterEVPNSetupScriptURL, env); err != nil {
		return fmt.Errorf("setup script failed: %w", err)
	}

	framework.Logf("Cluster-side EVPN setup complete")
	return nil
}

// runRemoteScript executes a bash script from a URL with the given environment variables.
// Uses: curl -sL $url | bash
func runRemoteScript(url string, env map[string]string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -sL %s | bash", url))

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		framework.Logf("Script output:\n%s", string(output))
	}
	if err != nil {
		return fmt.Errorf("script failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// copyEnvMap creates a copy of an environment variable map.
func copyEnvMap(m map[string]string) map[string]string {
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// =============================================================================
// EVPN Test Helpers
// =============================================================================

// =============================================================================
// EVPN Tests
// =============================================================================

var _ = ginkgo.Describe("EVPN", func() {
	const baseName = "evpn"
	f := wrappedTestFramework(baseName)
	f.SkipNamespaceCreation = true

	// Common test parameters
	const (
		// bgpASN is the general BGP ASN matching kind.sh's external FRR setup.
		// Used for: EVPN control plane, frr-k8s node peering, route-target calculation.
		bgpASN = 64512

		// ipVRFBGPASN is the BGP ASN for IP-VRF instances on the external FRR.
		// Different from bgpASN to enable eBGP for North-South traffic testing.
		// FRR supports multiple ASNs when each is in a different VRF.
		// See: https://docs.frrouting.org/en/stable-10.2/bgp.html#multiple-autonomous-systems
		ipVRFBGPASN = 64600

		// CUDN subnet - pods in the CUDN and MAC-VRF agnhost share this subnet
		// IPv4: /16 allows /24 per-node slices (256 nodes)
		// IPv6: /48 allows /64 per-node slices (65536 nodes)
		cudnSubnetIPv4 = "10.100.0.0/16"
		cudnSubnetIPv6 = "fd00:100::/48"

		// IP-VRF agnhost subnet - external network for L3 connectivity testing
		ipVRFAgnhostSubnetIPv4 = "172.27.102.0/24"
		ipVRFAgnhostSubnetIPv6 = "fd00:102::/64"

		// VTEP subnet for VTEP IP allocation
		vtepSubnetIPv4 = "100.64.0.0/24"
		vtepSubnetIPv6 = "fd00:64::/112"

		// Connectivity test parameters
		timeout = 15 * time.Second
	)
	netexecPortStr := fmt.Sprintf("%d", agnhostHTTPPort)

	var (
		ictx infraapi.Context

		// Test identifiers (set in BeforeEach with unique suffix)
		testBaseName string

		// External FRR IP (discovered in BeforeEach)
		externalFRRIP string

		// IP families supported by cluster (populated in BeforeEach)
		ipFamilies sets.Set[utilnet.IPFamily]

		// Subnets (dual-stack aware, populated in BeforeEach based on cluster IP family support)
		cudnSubnets         []string
		ipVRFAgnhostSubnets []string
		vtepSubnets         []string

		// Node IPs for BGP neighbors (populated in BeforeEach)
		nodeIPs []string
	)

	ginkgo.BeforeEach(func() {
		if !isLocalGWModeEnabled() {
			e2eskipper.Skipf("EVPN test cases only supported in Local Gateway mode")
		}
		if !isIPv4Supported(f.ClientSet) {
			// FRR does not support IPv6 underlay for EVPN VXLAN tunnels.
			// See: https://github.com/FRRouting/frr/issues/5885
			e2eskipper.Skipf("EVPN test cases require IPv4 for VXLAN underlay (FRR limitation)")
		}
		// Initialize infrastructure context (cleanup is automatic via ginkgo.DeferCleanup)
		ictx = infraprovider.Get().NewTestContext()

		// Generate unique test name with suffix
		testBaseName = baseName + framework.RandomSuffix()

		// Discover cluster IP family support
		ipFamilies = sets.New(getSupportedIPFamiliesSlice(f.ClientSet)...)

		// Reset slices for each test
		cudnSubnets = nil
		ipVRFAgnhostSubnets = nil
		vtepSubnets = nil
		nodeIPs = nil

		// Configure subnets based on cluster IP family support
		if ipFamilies.Has(utilnet.IPv4) {
			cudnSubnets = append(cudnSubnets, cudnSubnetIPv4)
			ipVRFAgnhostSubnets = append(ipVRFAgnhostSubnets, ipVRFAgnhostSubnetIPv4)
			vtepSubnets = append(vtepSubnets, vtepSubnetIPv4)
		}
		if ipFamilies.Has(utilnet.IPv6) {
			cudnSubnets = append(cudnSubnets, cudnSubnetIPv6)
			ipVRFAgnhostSubnets = append(ipVRFAgnhostSubnets, ipVRFAgnhostSubnetIPv6)
			vtepSubnets = append(vtepSubnets, vtepSubnetIPv6)
		}

		// Discover external FRR IP
		kindNetwork, err := infraprovider.Get().GetNetwork("kind")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		frrNetIf, err := infraprovider.Get().GetExternalContainerNetworkInterface(
			infraapi.ExternalContainer{Name: externalFRRContainerName}, kindNetwork)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Use IPv4 or IPv6 based on cluster support (prefer IPv4 if both available)
		if isIPv4Supported(f.ClientSet) && frrNetIf.IPv4 != "" {
			externalFRRIP = frrNetIf.IPv4
		} else if isIPv6Supported(f.ClientSet) && frrNetIf.IPv6 != "" {
			externalFRRIP = frrNetIf.IPv6
		}
		gomega.Expect(externalFRRIP).NotTo(gomega.BeEmpty(), "External FRR must have an IP on kind network")
		framework.Logf("External FRR IP: %s", externalFRRIP)

		// Collect node IPs for BGP neighbors
		nodes, err := f.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		for _, node := range nodes.Items {
			if ipFamilies.Has(utilnet.IPv4) {
				addrs := e2enode.GetAddressesByTypeAndFamily(&node, corev1.NodeInternalIP, corev1.IPv4Protocol)
				nodeIPs = append(nodeIPs, addrs...)
			}
			if ipFamilies.Has(utilnet.IPv6) {
				addrs := e2enode.GetAddressesByTypeAndFamily(&node, corev1.NodeInternalIP, corev1.IPv6Protocol)
				nodeIPs = append(nodeIPs, addrs...)
			}
		}
		framework.Logf("Node IPs for BGP: %v", nodeIPs)
	})

	// Helper to test connectivity from pod to external server (handles both IPv4 and IPv6)
	testPodToServers := func(pod *corev1.Pod, serverIPs []string) {
		ginkgo.GinkgoHelper()
		for _, serverIP := range serverIPs {
			// Strip prefix length if present (e.g., "10.100.0.250/16" -> "10.100.0.250")
			ip := strings.Split(serverIP, "/")[0]
			url := fmt.Sprintf("http://%s/hostname", net.JoinHostPort(ip, netexecPortStr))
			framework.Logf("Testing connectivity to %s", url)
			_, err := e2epodoutput.RunHostCmdWithRetries(
				pod.Namespace,
				pod.Name,
				fmt.Sprintf("curl --max-time 1 -g -q -s %s", url),
				framework.Poll,
				timeout,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to connect to %s", serverIP)
		}
	}

	// Note: Cleanup is automatic via ginkgo.DeferCleanup registered in NewTestContext()
	// All functions using ictx.AddCleanUpFn() will be cleaned up in LIFO order

	ginkgo.DescribeTable("EVPN connectivity", feature.RouteAdvertisements, feature.EVPN,
		func(networkSpec *udnv1.NetworkSpec) {
			// Derive what to setup from networkSpec
			hasMACVRF := networkSpec.EVPN != nil && networkSpec.EVPN.MACVRF != nil
			hasIPVRF := networkSpec.EVPN != nil && networkSpec.EVPN.IPVRF != nil

			// Filter networkSpec subnets based on cluster IP family support
			if networkSpec.Layer2 != nil {
				networkSpec.Layer2.Subnets = filterDualStackCIDRs(f.ClientSet, networkSpec.Layer2.Subnets)
			}
			if networkSpec.Layer3 != nil {
				networkSpec.Layer3.Subnets = filterL3Subnets(f.ClientSet, networkSpec.Layer3.Subnets)
			}

			// Extract subnets from networkSpec for MAC-VRF agnhost IP derivation
			var cudnSubnetsFromSpec []string
			if networkSpec.Layer2 != nil {
				for _, cidr := range networkSpec.Layer2.Subnets {
					cudnSubnetsFromSpec = append(cudnSubnetsFromSpec, string(cidr))
				}
			} else if networkSpec.Layer3 != nil {
				for _, subnet := range networkSpec.Layer3.Subnets {
					cudnSubnetsFromSpec = append(cudnSubnetsFromSpec, string(subnet.CIDR))
				}
			}

			ginkgo.By("Setting up EVPN bridge on external FRR")
			gomega.Expect(setupEVPNBridgeOnExternalFRR(ictx, externalFRRIP)).To(gomega.Succeed())

			var macVRFAgnhostIPs []string
			if hasMACVRF {
				// Generate random VIDs for external FRR side.
				// VID is local to each side - only VNI needs to match for EVPN connectivity.
				frrMACVRFVID := randomVID()
				framework.Logf("Generated random VIDs for external FRR: MAC-VRF VID=%d", frrMACVRFVID)
				ginkgo.By("Setting up MAC-VRF on external FRR")
				gomega.Expect(setupMACVRFOnExternalFRR(int(networkSpec.EVPN.MACVRF.VNI), frrMACVRFVID)).To(gomega.Succeed())

				ginkgo.By("Creating MAC-VRF agnhost")
				var err error
				macVRFAgnhostIPs, err = setupMACVRFAgnhost(ictx, frrMACVRFVID, ipFamilies, cudnSubnetsFromSpec...)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(macVRFAgnhostIPs).NotTo(gomega.BeEmpty())
			}
			ginkgo.By("Setting up EVPN BGP on external FRR")
			gomega.Expect(setupEVPNBGPOnExternalFRR(ictx, bgpASN, nodeIPs)).To(gomega.Succeed())

			var ipVRFAgnhostIPs []string
			if hasIPVRF {
				// Derive VRF name from VNI (unique per IP-VRF)
				ipVRFName := fmt.Sprintf("vrf%d", networkSpec.EVPN.IPVRF.VNI)
				frrIPVRFVID := randomVID()
				framework.Logf("Generated random VIDs for external FRR: IP-VRF VID=%d", frrIPVRFVID)
				ginkgo.By("Setting up IP-VRF on external FRR")
				gomega.Expect(setupIPVRFOnExternalFRR(ictx, ipVRFName, int(networkSpec.EVPN.IPVRF.VNI), frrIPVRFVID)).To(gomega.Succeed())
				ginkgo.By("Creating IP-VRF agnhost")
				var err error
				ipVRFAgnhostIPs, err = setupIPVRFAgnhost(ictx, frrIPVRFVID, ipVRFName, ipFamilies, ipVRFAgnhostSubnets...)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(ipVRFAgnhostIPs).NotTo(gomega.BeEmpty())
				// Configure BGP AFTER agnhost so FRR's interface is in the VRF
				// and has a connected route for the subnet we want to advertise
				ginkgo.By("Setting up IP-VRF BGP on external FRR")
				// ipVRFBGPASN (64600) for VRF BGP instance, bgpASN (64512) for route-target to match cluster-side
				gomega.Expect(setupIPVRFBGPOnExternalFRR(ictx, ipVRFName, ipVRFBGPASN, bgpASN, int(networkSpec.EVPN.IPVRF.VNI), ipFamilies, ipVRFAgnhostSubnets)).To(gomega.Succeed())
			}

			ginkgo.By("Creating VTEP CR")
			testVTEPName := testBaseName + "-vtep"
			err := createVTEP(f, ictx, testVTEPName, vtepSubnets, vtepv1.VTEPModeManaged)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Update VTEP name in network spec
			networkSpec.EVPN.VTEP = testVTEPName

			ginkgo.By("Creating FRRConfiguration for EVPN")
			frrConfigLabels := map[string]string{"network": testBaseName}
			err = createFRRConfiguration(ictx, testBaseName, deploymentconfig.Get().FRRK8sNamespace(), bgpASN, externalFRRIP, frrConfigLabels)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Creating namespace, CUDN, and RouteAdvertisements")
			testNamespace, err := createNamespaceWithPrimaryNetworkOfType(f, ictx, baseName, testBaseName, cudnAdvertisedEVPN, networkSpec)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			f.Namespace = testNamespace

			// REVERT ME: Temporary cluster-side EVPN setup until OVN-K implements it natively
			// Generate random VIDs for cluster-side EVPN setup (OVN-K side).
			// VID is local to each side - these don't need to match the FRR side VIDs.
			clusterMACVRFVID := randomVID()
			clusterIPVRFVID := randomVID()
			framework.Logf("Generated random VIDs for cluster side: MAC-VRF VID=%d, IP-VRF VID=%d", clusterMACVRFVID, clusterIPVRFVID)
			ginkgo.By("Running cluster-side EVPN setup script (REVERT ME)")
			var macVRFVNI, ipVRFVNI int
			if hasMACVRF {
				macVRFVNI = int(networkSpec.EVPN.MACVRF.VNI)
			}
			if hasIPVRF {
				ipVRFVNI = int(networkSpec.EVPN.IPVRF.VNI)
			}
			err = runClusterEVPNSetupScript(ictx,
				testBaseName, externalFRRIP, bgpASN,
				hasMACVRF, macVRFVNI, clusterMACVRFVID,
				hasIPVRF, ipVRFVNI, clusterIPVRFVID,
				cudnSubnetsFromSpec)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Creating test pod on CUDN")
			testPod := e2epod.CreateExecPodOrFail(
				context.Background(),
				f.ClientSet,
				f.Namespace.Name,
				f.Namespace.Name+"-netexec-pod",
				func(p *corev1.Pod) {
					p.Spec.Containers[0].Args = []string{"netexec"}
				},
			)

			// Test connectivity to external servers
			if hasMACVRF {
				framework.Logf("Testing L2 connectivity to MAC-VRF agnhost IPs: %v", macVRFAgnhostIPs)
				testPodToServers(testPod, macVRFAgnhostIPs)
			}
			if hasIPVRF {
				framework.Logf("Testing L3 connectivity to IP-VRF agnhost IPs: %v", ipVRFAgnhostIPs)
				testPodToServers(testPod, ipVRFAgnhostIPs)
			}
		},
		// Layer2 with MAC-VRF only - L2 broadcast domain extended via EVPN
		ginkgo.Entry("Layer2 network with MAC-VRF",
			&udnv1.NetworkSpec{
				Topology: udnv1.NetworkTopologyLayer2,
				Layer2: &udnv1.Layer2Config{
					Role:    udnv1.NetworkRolePrimary,
					Subnets: udnv1.DualStackCIDRs{udnv1.CIDR(cudnSubnetIPv4), udnv1.CIDR(cudnSubnetIPv6)},
				},
				Transport: udnv1.TransportOptionEVPN,
				EVPN: &udnv1.EVPNConfig{
					MACVRF: &udnv1.VRFConfig{
						VNI: randomVNI(),
					},
				},
			},
		),
		// Layer2 with MAC-VRF + IP-VRF - L2 domain with L3 routing to external
		ginkgo.Entry("Layer2 network with MAC-VRF and IP-VRF",
			&udnv1.NetworkSpec{
				Topology: udnv1.NetworkTopologyLayer2,
				Layer2: &udnv1.Layer2Config{
					Role:    udnv1.NetworkRolePrimary,
					Subnets: udnv1.DualStackCIDRs{udnv1.CIDR(cudnSubnetIPv4), udnv1.CIDR(cudnSubnetIPv6)},
				},
				Transport: udnv1.TransportOptionEVPN,
				EVPN: &udnv1.EVPNConfig{
					MACVRF: &udnv1.VRFConfig{
						VNI: randomVNI(),
					},
					IPVRF: &udnv1.VRFConfig{
						VNI: randomVNI(),
					},
				},
			},
		),
		// Layer3 with IP-VRF only - L3 routing via EVPN Type-5 routes
		ginkgo.Entry("Layer3 network with IP-VRF",
			&udnv1.NetworkSpec{
				Topology: udnv1.NetworkTopologyLayer3,
				Layer3: &udnv1.Layer3Config{
					Role: udnv1.NetworkRolePrimary,
					Subnets: []udnv1.Layer3Subnet{
						{CIDR: udnv1.CIDR(cudnSubnetIPv4)},
						{CIDR: udnv1.CIDR(cudnSubnetIPv6)},
					},
				},
				Transport: udnv1.TransportOptionEVPN,
				EVPN: &udnv1.EVPNConfig{
					IPVRF: &udnv1.VRFConfig{
						VNI: randomVNI(),
					},
				},
			},
		),
	)
})
