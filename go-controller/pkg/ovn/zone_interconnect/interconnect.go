package zoneinterconnect

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"

	libovsdbclient "github.com/ovn-org/libovsdb/client"

	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdbops"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/nbdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/sbdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
)

const (
	transitSwitchTunnelKey = "16711683"
	lportTypeRouter        = "router"
	lportTypeRouterAddr    = "router"
	lportTypeRemote        = "remote"
)

/*
 * ZoneInterconnectHandler creates the OVN resources required for interconnecting
 * multiple zones. This handler exposes 2 main functions which a network controller
 * (default and secondary) is expected to call for a node event.
 * AddLocalZoneNode(node) should be called if the node 'node' is a local zone node.
 * AddRemoteZoneNode(node) should be called if the node 'node' is a remote zone node.
 * Zone Interconnect Handler first creates a transit switch with the name - <network_name>+ "_" + types.TransitSwitch
 * if it is still not present.
 *
 * Local zone node handling
 * ------------------------
 * When network controller calls AddLocalZoneNode(ovn-worker)
 *    -  A logical switch port - router port pair is created connecting the ovn_cluster_router
 *       to the transit switch.
 *    -  Node annotation - k8s.ovn.org/ovn-node-transit-switch-port-ifaddr value is used
 *       as the logical router port address
 *
 * When network controller calls AddRemoteZoneNode(ovn-worker3)
 *    - A logical switch port of type "remote" is created in OVN Northbound transit_switch
 *      for the node ovn-worker3
 *    - A static route {IPPrefix: "ovn-worker3_subnet", Nexthop: "ovn-worker3_transit_port_ip"} is
 *      added in the ovn_cluster_router.
 *    - For the default network, additional static route
 *      {IPPrefix: "ovn-worker3_gw_router_port_host_ip", Nexthop: "ovn-worker3_transit_port_ip"} is
 *      added in the ovn_cluster_router
 *    - The corresponding port binding row in OVN Southbound DB for this logical port
 *      is manually bound to the remote OVN Southbound DB Chassis "ovn-worker3"
 *
 * -----------------------------------------------------------------------------------------------------
 * $ ovn-nbctl show ovn_cluster_router (on ovn-worker zone DB)
 *   router ovn_cluster_router
 *   ...
 *   port rtots-ovn-worker
 *      mac: "0a:58:a8:fe:00:08"
 *      networks: ["168.254.0.8/16", "fd97::8/64"]
 *
 * $ ovn-nbctl show transit_switch
 *     port tstor-ovn-worker
 *        type: router
 *        router-port: rtots-ovn-worker
 *     port tstor-ovn-worker3
 *        type: remote
 *        addresses: ["0a:58:a8:fe:00:02 168.254.0.2/16 fd97::2/64"]
 *
 * $ ovn-nbctl lr-route-list ovn_cluster_router
 *    IPv4 Routes
 *    Route Table <main>:
 *    ...
 *    ...
 *    10.244.0.0/24 (ovn-worker3 subnet)            168.254.0.2 (ovn-worker3 transit switch port ip) dst-ip
 *    100.64.0.2/32 (ovn-worker3 gw router port ip) 168.254.0.2 dst-ip
 *    ...
 *    IPv6 Routes
 *    Route Table <main>:
 *    ...
 *    ...
 *    fd00:10:244:1::/64 (ovn-worker3 subnet)       fd97::2 (ovn-worker3 transit switch port ip) dst-ip
 *    fd98::2 (ovn-worker3 gw router port ip)       fd97::2 dst-ip
 *    ...
 *
 * $ ovn-sbctl show
 *     ...
 *     Chassis "c391c626-e1f0-4b1e-af0b-66f0807f9495"
 *     hostname: ovn-worker3 (Its a remote chassis entry on which tstor-ovn-worker3 is bound)
 *     Encap geneve
 *         ip: "10.89.0.26"
 *         options: {csum="true"}
 *     Port_Binding tstor-ovn-worker3
 *
 * -----------------------------------------------------------------------------------------------------
 *
 * Note that the Chassis entry for each remote zone node is created by ZoneChassisHandler
 *
 */

// ZoneInterconnectHandler creates the OVN resources required for interconnecting
// multiple zones for a network (default or secondary layer 3)
type ZoneInterconnectHandler struct {
	// network which is inter-connected
	util.NetInfo
	nbClient libovsdbclient.Client
	sbClient libovsdbclient.Client
	// ovn_cluster_router name for the network
	networkClusterRouterName string
	// transit switch name for the network
	networkTransitSwitchName string
}

func NewZoneInterconnectHandler(nInfo util.NetInfo, nbClient, sbClient libovsdbclient.Client) *ZoneInterconnectHandler {
	zic := &ZoneInterconnectHandler{
		NetInfo:  nInfo,
		nbClient: nbClient,
		sbClient: sbClient,
	}

	zic.networkClusterRouterName = zic.getNetworkScopedName(types.OVNClusterRouter)
	zic.networkTransitSwitchName = zic.getNetworkScopedName(types.TransitSwitch)

	return zic
}

// AddLocalZoneNode creates the interconnect resources in OVN NB DB for the local zone node.
// See createLocalZoneNodeResources() below for more details.
func (zic *ZoneInterconnectHandler) AddLocalZoneNode(node *corev1.Node) error {
	klog.Infof("Creating interconnect resources for local zone node [%s] for the network [%s]", node.Name, zic.GetNetworkName())
	nodeID := util.GetNodeID(node)
	if nodeID == -1 {
		// Don't consider this node as cluster-manager has not allocated node id yet.
		return fmt.Errorf("failed to get node id for node - %s", node.Name)
	}

	if err := zic.createLocalZoneNodeResources(node, nodeID); err != nil {
		klog.Errorf("Creating interconnect resources for local zone node [%s] for the network [%s] failed : err - %w", node.Name, zic.GetNetworkName(), err)
		return fmt.Errorf("creating interconnect resources for local zone node [%s] for the network [%s] failed : err - %w", node.Name, zic.GetNetworkName(), err)
	}

	return nil
}

// AddRemoteZoneNode creates the interconnect resources in OVN NB DB for the remote zone node.
// // See createRemoteZoneNodeResources() below for more details.
func (zic *ZoneInterconnectHandler) AddRemoteZoneNode(node *corev1.Node) error {
	klog.Infof("Creating interconnect resources for remote zone node [%s] for the network [%s]", node.Name, zic.GetNetworkName())

	nodeID := util.GetNodeID(node)
	if nodeID == -1 {
		// Don't consider this node as cluster-manager has not allocated node id yet.
		return fmt.Errorf("failed to get node id for node - %s", node.Name)
	}

	// Get the chassis id.
	chassisId, err := util.ParseNodeChassisIDAnnotation(node)
	if err != nil {
		return fmt.Errorf("failed to parse node chassis-id for node - %s, error: %w", node.Name, err)
	}

	if err := zic.createRemoteZoneNodeResources(node, nodeID, chassisId); err != nil {
		klog.Errorf("Creating interconnect resources for remote zone node [%s] for the network [%s] failed : err - %w", node.Name, zic.GetNetworkName(), err)
		return fmt.Errorf("creating interconnect resources for remote zone node [%s] for the network [%s] failed : err - %w", node.Name, zic.GetNetworkName(), err)
	}

	return nil
}

// DeleteNode deletes the local zone node or remote zone node resources
func (zic *ZoneInterconnectHandler) DeleteNode(node *corev1.Node) error {
	klog.Infof("Deleting interconnect resources for the node [%s] for the network [%s]", node.Name, zic.GetNetworkName())

	// Cleanup the logical router port in the cluster router for the node
	// if it exists.
	if err := zic.cleanupNodeClusterRouterPort(node); err != nil {
		return err
	}

	// Cleanup the logical switch port in the transit switch for the node
	// if it exists.
	return zic.cleanupNodeTransitSwitchPort(node)
}

// createLocalZoneNodeResources creates the local zone node resources for interconnect
//   - creates Transit switch if it doesn't yet exit
//   - creates a logical switch port of type "router" in the transit switch with the name as - <network_name>.tstor-<node_name>
//     Eg. if the node name is ovn-worker and the network is default, the name would be - tstor-ovn-worker
//     if the node name is ovn-worker and the network name is blue, the logical port name would be - blue.tstor-ovn-worker
//   - creates a logical router port in the ovn_cluster_router with the name - <network_name>.rtots-<node_name> and connects
//     to the node logical switch port in the transit switch
//   - remove any stale static routes in the ovn_cluster_router for the node
func (zic *ZoneInterconnectHandler) createLocalZoneNodeResources(node *corev1.Node, nodeID int) error {
	nodeTransitSwitchPortIPs, err := util.ParseNodeTransitSwitchPortAddrs(node)
	if err != nil || len(nodeTransitSwitchPortIPs) == 0 {
		return fmt.Errorf("failed to get the node transit switch port ips for node %s: %w", node.Name, err)
	}

	transitRouterPortMac := util.IPAddrToHWAddr(nodeTransitSwitchPortIPs[0].IP)
	var transitRouterPortNetworks []string
	for _, ip := range nodeTransitSwitchPortIPs {
		transitRouterPortNetworks = append(transitRouterPortNetworks, ip.String())
	}

	ts := &nbdb.LogicalSwitch{
		Name: zic.networkTransitSwitchName,
	}

	_, err = libovsdbops.GetLogicalSwitch(zic.nbClient, ts)
	if err != nil {
		if errors.Is(err, libovsdbclient.ErrNotFound) {
			// Create transit switch
			ts.OtherConfig = map[string]string{
				"interconn-ts":             zic.networkTransitSwitchName,
				"requested-tnl-key":        transitSwitchTunnelKey,
				"mcast_snoop":              "true",
				"mcast_flood_unregistered": "true",
			}
			if err := libovsdbops.CreateOrUpdateLogicalSwitch(zic.nbClient, ts); err != nil {
				return fmt.Errorf("failed to create/update logical transit switch: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get the transit switch %s from the libovsdb cache : %w", zic.networkTransitSwitchName, err)
		}
	}

	// Connect transit switch to the cluster router by creating a pair of logical switch port - logical router port
	logicalRouterPortName := zic.getNetworkScopedName(types.RouterToTransitSwitchPrefix + node.Name)
	logicalRouterPort := nbdb.LogicalRouterPort{
		Name:     logicalRouterPortName,
		MAC:      transitRouterPortMac.String(),
		Networks: transitRouterPortNetworks,
		Options: map[string]string{
			"mcast_flood": "true",
		},
	}
	logicalRouter := nbdb.LogicalRouter{
		Name: zic.networkClusterRouterName,
	}

	if err := libovsdbops.CreateOrUpdateLogicalRouterPort(zic.nbClient, &logicalRouter,
		&logicalRouterPort, nil); err != nil {
		return fmt.Errorf("failed to create/update cluster router %s to add transit switch port %s for the node %s: %w", zic.networkClusterRouterName, logicalRouterPortName, node.Name, err)
	}

	lspOptions := map[string]string{
		"router-port":       logicalRouterPortName,
		"requested-tnl-key": strconv.Itoa(nodeID),
	}

	err = zic.addNodeLogicalSwitchPort(zic.networkTransitSwitchName, zic.getNetworkScopedName(types.TransitSwitchToRouterPrefix+node.Name),
		lportTypeRouter, []string{lportTypeRouterAddr}, lspOptions)
	if err != nil {
		return err
	}

	// Its possible that node is moved from a remote zone to the local zone. Check and delete the remote zone routes
	// for this node as it's no longer needed.
	return zic.deleteLocalNodeStaticRoutes(node, nodeID, nodeTransitSwitchPortIPs)
}

// createRemoteZoneNodeResources creates the remote zone node resources
//   - creates Transit switch if it doesn't yet exit
//   - creates a logical port of type "remote" in the transit switch with the name as - <network_name>.tstor.<node_name>
//     Eg. if the node name is ovn-worker and the network is default, the name would be - tstor.ovn-worker
//     if the node name is ovn-worker and the network name is blue, the logical port name would be - blue.tstor.ovn-worker
//   - binds the remote port to the node remote chassis
//   - adds static routes for the remote node via the remote port ip in the ovn_cluster_router
func (zic *ZoneInterconnectHandler) createRemoteZoneNodeResources(node *corev1.Node, nodeID int, chassisId string) error {
	nodeTransitSwitchPortIPs, err := util.ParseNodeTransitSwitchPortAddrs(node)
	if err != nil || len(nodeTransitSwitchPortIPs) == 0 {
		return fmt.Errorf("failed to get the node transit switch port Ips : %w", err)
	}

	ts := &nbdb.LogicalSwitch{
		Name: zic.networkTransitSwitchName,
	}

	_, err = libovsdbops.GetLogicalSwitch(zic.nbClient, ts)
	if err != nil {
		if errors.Is(err, libovsdbclient.ErrNotFound) {
			// Create transit switch
			ts.OtherConfig = map[string]string{
				"interconn-ts":             zic.networkTransitSwitchName,
				"requested-tnl-key":        transitSwitchTunnelKey,
				"mcast_snoop":              "true",
				"mcast_flood_unregistered": "true",
			}
			if err := libovsdbops.CreateOrUpdateLogicalSwitch(zic.nbClient, ts); err != nil {
				return fmt.Errorf("failed to create/update logical transit switch: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get the transit switch %s from the libovsdb cache : %w", zic.networkTransitSwitchName, err)
		}
	}

	transitRouterPortMac := util.IPAddrToHWAddr(nodeTransitSwitchPortIPs[0].IP)
	var transitRouterPortNetworks []string
	for _, ip := range nodeTransitSwitchPortIPs {
		transitRouterPortNetworks = append(transitRouterPortNetworks, ip.String())
	}

	remotePortAddr := transitRouterPortMac.String()
	for _, tsNetwork := range transitRouterPortNetworks {
		remotePortAddr = remotePortAddr + " " + tsNetwork
	}

	lspOptions := map[string]string{
		"requested-tnl-key": strconv.Itoa(nodeID),
	}
	remotePortName := zic.getNetworkScopedName(types.TransitSwitchToRouterPrefix + node.Name)
	if err := zic.addNodeLogicalSwitchPort(zic.networkTransitSwitchName, remotePortName, lportTypeRemote, []string{remotePortAddr}, lspOptions); err != nil {
		return err
	}
	// Set the port binding chassis.
	if err := zic.setRemotePortBindingChassis(node.Name, remotePortName, chassisId); err != nil {
		return err
	}

	if err := zic.addRemoteNodeStaticRoutes(node, nodeID, nodeTransitSwitchPortIPs); err != nil {
		return err
	}

	// Cleanup the logical router port connecting to the transit switch for the remote node (if present)
	// Cleanup would be required when a local zone node moves to a remote zone.
	return zic.cleanupNodeClusterRouterPort(node)
}

func (zic *ZoneInterconnectHandler) addNodeLogicalSwitchPort(logicalSwitchName, portName, portType string, addresses []string, options map[string]string) error {
	logicalSwitch := nbdb.LogicalSwitch{
		Name: logicalSwitchName,
	}

	logicalSwitchPort := nbdb.LogicalSwitchPort{
		Name:      portName,
		Type:      portType,
		Options:   options,
		Addresses: addresses,
	}
	if err := libovsdbops.CreateOrUpdateLogicalSwitchPortsOnSwitch(zic.nbClient, &logicalSwitch, &logicalSwitchPort); err != nil {
		return fmt.Errorf("failed to add logical port %s to switch %s, error: %w", portName, logicalSwitch.Name, err)
	}
	return nil
}

func (zic *ZoneInterconnectHandler) cleanupNodeClusterRouterPort(node *corev1.Node) error {
	lrp := nbdb.LogicalRouterPort{
		Name: zic.getNetworkScopedName(types.RouterToTransitSwitchPrefix + node.Name),
	}
	logicalRouterPort, err := libovsdbops.GetLogicalRouterPort(zic.nbClient, &lrp)
	if err != nil {
		// logical router port doesn't exist. So nothing to cleanup.
		return nil
	}

	logicalRouter := nbdb.LogicalRouter{
		Name: zic.networkClusterRouterName,
	}

	if err := libovsdbops.DeleteLogicalRouterPorts(zic.nbClient, &logicalRouter, logicalRouterPort); err != nil {
		return fmt.Errorf("failed to delete logical router port %s from router %s for the node %s, error: %w", logicalRouterPort.Name, zic.networkClusterRouterName, node.Name, err)
	}

	return nil
}

func (zic *ZoneInterconnectHandler) cleanupNodeTransitSwitchPort(node *corev1.Node) error {
	logicalSwitch := &nbdb.LogicalSwitch{
		Name: zic.networkTransitSwitchName,
	}
	logicalSwitchPort := &nbdb.LogicalSwitchPort{
		Name: zic.getNetworkScopedName(types.TransitSwitchToRouterPrefix + node.Name),
	}

	if err := libovsdbops.DeleteLogicalSwitchPorts(zic.nbClient, logicalSwitch, logicalSwitchPort); err != nil {
		return fmt.Errorf("failed to delete logical switch port %s from transit switch %s for the node %s, error: %w", logicalSwitchPort.Name, zic.networkTransitSwitchName, node.Name, err)
	}
	return nil
}

func (zic *ZoneInterconnectHandler) setRemotePortBindingChassis(nodeName, portName, chassisId string) error {
	remotePort := sbdb.PortBinding{
		LogicalPort: portName,
	}
	chassis := sbdb.Chassis{
		Hostname: nodeName,
		Name:     chassisId,
	}

	if err := libovsdbops.UpdatePortBindingSetChassis(zic.sbClient, &remotePort, &chassis); err != nil {
		return fmt.Errorf("failed to update chassis %s for remote port %s, error: %w", nodeName, portName, err)
	}

	return nil
}

func (zic *ZoneInterconnectHandler) addRemoteNodeStaticRoutes(node *corev1.Node, nodeID int, nodeTransitSwitchPortIPs []*net.IPNet) error {
	addRoute := func(subnet *net.IPNet, tsIp string) error {
		logicalRouterStaticRoute := nbdb.LogicalRouterStaticRoute{
			ExternalIDs: map[string]string{
				"ic-node": node.Name,
			},
			Nexthop:  tsIp,
			IPPrefix: subnet.String(),
		}
		p := func(lrsr *nbdb.LogicalRouterStaticRoute) bool {
			return lrsr.IPPrefix == subnet.String() &&
				lrsr.Nexthop == tsIp &&
				lrsr.ExternalIDs["ic-node"] == node.Name
		}
		//FIXME: All routes should be added in a single transaction instead.
		if err := libovsdbops.CreateOrReplaceLogicalRouterStaticRouteWithPredicate(zic.nbClient, zic.networkClusterRouterName, &logicalRouterStaticRoute, p); err != nil {
			return fmt.Errorf("failed to create static route: %w", err)
		}
		return nil
	}

	nodeSubnets, err := util.ParseNodeHostSubnetAnnotation(node, zic.GetNetworkName())
	if err != nil {
		return fmt.Errorf("failed to parse node %s subnets annotation %w", node.Name, err)
	}

	//FIXME: No consistency on transaction failure.
	for _, subnet := range nodeSubnets {
		for _, tsIP := range nodeTransitSwitchPortIPs {
			if utilnet.IPFamilyOfCIDR(subnet) != utilnet.IPFamilyOfCIDR(tsIP) {
				continue
			}
			if err := addRoute(subnet, tsIP.IP.String()); err != nil {
				return fmt.Errorf("unable to create static routes: %w", err)
			}

		}
	}

	if zic.IsSecondary() {
		// Secondary network cluster router doesn't connect to a join switch
		// or to a Gateway router.
		return nil
	}

	nodeGRPIPs, err := util.ParseNodeGatewayRouterLRPAddrs(node)
	if err != nil {
		return fmt.Errorf("failed to parse node %s Gateway router LRP Addrs annotation %w", node.Name, err)
	}

	for _, gwRPAddr := range nodeGRPIPs {
		for _, tsIP := range nodeTransitSwitchPortIPs {
			if utilnet.IPFamilyOfCIDR(gwRPAddr) != utilnet.IPFamilyOfCIDR(tsIP) {
				continue
			}
			hostRoute := &net.IPNet{IP: gwRPAddr.IP}
			if utilnet.IsIPv4CIDR(gwRPAddr) {
				hostRoute.Mask = net.CIDRMask(32, 32)
			} else {
				hostRoute.Mask = net.CIDRMask(128, 128)
			}
			if err := addRoute(hostRoute, tsIP.IP.String()); err != nil {
				return fmt.Errorf("unable to create static routes: %w", err)
			}
		}
	}

	return nil
}

func (zic *ZoneInterconnectHandler) deleteLocalNodeStaticRoutes(node *corev1.Node, nodeID int, nodeTransitSwitchPortIPs []*net.IPNet) error {
	deleteRoute := func(subnet *net.IPNet, tsIp string) error {
		p := func(lrsr *nbdb.LogicalRouterStaticRoute) bool {
			return lrsr.IPPrefix == subnet.String() &&
				lrsr.Nexthop == tsIp &&
				lrsr.ExternalIDs["ic-node"] == node.Name
		}
		//FIXME: All routes should be added in a single transaction instead.
		if err := libovsdbops.DeleteLogicalRouterStaticRoutesWithPredicate(zic.nbClient, zic.networkClusterRouterName, p); err != nil {
			return fmt.Errorf("failed to delete static route: %w", err)
		}
		return nil
	}

	nodeSubnets, err := util.ParseNodeHostSubnetAnnotation(node, zic.GetNetworkName())
	if err != nil {
		return fmt.Errorf("failed to parse node %s subnets annotation %w", node.Name, err)
	}

	for _, subnet := range nodeSubnets {
		for _, tsIP := range nodeTransitSwitchPortIPs {
			if utilnet.IPFamilyOfCIDR(subnet) != utilnet.IPFamilyOfCIDR(tsIP) {
				continue
			}
			if err := deleteRoute(subnet, tsIP.IP.String()); err != nil {
				return fmt.Errorf("unable to create static routes: %w", err)
			}

		}
	}

	if zic.IsSecondary() {
		// Secondary network cluster router doesn't connect to a join switch
		// or to a Gateway router.
		return nil
	}

	// Clear the routes connecting to the GW Router for the default network
	nodeGWRPIPs, err := util.ParseNodeGatewayRouterLRPAddrs(node)
	if err != nil {
		return fmt.Errorf("failed to parse node %s Gateway router LRP Addrs annotation %w", node.Name, err)
	}

	for _, gwRPAddr := range nodeGWRPIPs {
		for _, tsIP := range nodeTransitSwitchPortIPs {
			if utilnet.IPFamilyOfCIDR(gwRPAddr) != utilnet.IPFamilyOfCIDR(tsIP) {
				continue
			}
			hostRoute := &net.IPNet{IP: gwRPAddr.IP}
			if utilnet.IsIPv4CIDR(gwRPAddr) {
				hostRoute.Mask = net.CIDRMask(32, 32)
			} else {
				hostRoute.Mask = net.CIDRMask(128, 128)
			}
			if err := deleteRoute(hostRoute, tsIP.IP.String()); err != nil {
				return fmt.Errorf("unable to create static routes: %w", err)
			}
		}
	}

	return nil
}

func (zic *ZoneInterconnectHandler) getNetworkScopedName(name string) string {
	return fmt.Sprintf("%s%s", zic.GetPrefix(), name)
}
