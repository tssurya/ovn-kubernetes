package networkconnect

import (
	"errors"
	"fmt"
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"

	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/clustermanager/node"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	networkconnectv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1"
	apitypes "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/types"
	ovntypes "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
)

var (
	errConfig         = errors.New("configuration error")
	p2pIPV4SubnetMask = 31
	p2pIPV6SubnetMask = 127
)

func (c *Controller) reconcileClusterNetworkConnect(key string) error {
	startTime := time.Now()
	_, cncName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	klog.V(5).Infof("reconcileClusterNetworkConnect %s", cncName)
	defer func() {
		klog.Infof("reconcileClusterNetworkConnect %s took %v", cncName, time.Since(startTime))
	}()
	cnc, err := c.cncLister.Get(cncName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if cnc == nil {
		// TODO: delete the CNC and related resources
		return nil
	}

	// STEP1: Validate the CNC
	// STEP2: Discover the selected UDNs and CUDNs
	discoveredLayer3Networks, discoveredLayer2Networks, err := c.discoverSelectedNetworks(cnc)
	if err != nil {
		return err
	}
	if len(discoveredLayer3Networks) == 0 && len(discoveredLayer2Networks) == 0 {
		klog.Infof("no networks found for CNC %s", cncName)
		return nil
	}
	// STEP3: Generate /31 subnets for each node-network pair for layer3 networks and one /31 subnet for each layer2 networks
	allocatedSubnets, err := c.allocateSubnets(discoveredLayer3Networks, discoveredLayer2Networks, cnc.Spec.ConnectSubnets, cncName)
	if err != nil {
		return err
	}
	err = util.UpdateNetworkConnectSubnetAnnotation(cnc, c.cncClient, allocatedSubnets)
	if err != nil {
		return err
	}
	// STEP4: Generate a tunnelID for the connect router corresponding to this CNC

	return nil
}

func (c *Controller) discoverSelectedNetworks(cnc *networkconnectv1.ClusterNetworkConnect) ([]*util.NetInfo, []*util.NetInfo, error) {
	discoveredLayer3Networks := []*util.NetInfo{}
	discoveredLayer2Networks := []*util.NetInfo{}

	for _, selector := range cnc.Spec.NetworkSelectors {
		switch selector.NetworkSelectionType {
		case apitypes.ClusterUserDefinedNetworks:
			networkSelector, err := metav1.LabelSelectorAsSelector(&selector.ClusterUserDefinedNetworkSelector.NetworkSelector)
			if err != nil {
				return nil, nil, err
			}
			nads, err := c.nadLister.List(networkSelector)
			if err != nil {
				return nil, nil, err
			}
			for _, nad := range nads {
				// check this NAD is controlled by a CUDN
				controller := metav1.GetControllerOfNoCopy(nad)
				isCUDN := controller != nil && controller.Kind == cudnController.Kind && controller.APIVersion == cudnController.GroupVersion().String()
				isUDN := controller != nil && controller.Kind == udnController.Kind && controller.APIVersion == udnController.GroupVersion().String()
				if !isCUDN && !isUDN {
					continue
				}
				network, err := util.ParseNADInfo(nad)
				if err != nil {
					return nil, nil, err
				}
				if !network.IsPrimaryNetwork() {
					continue
				}
				if network.TopologyType() == ovntypes.Layer3Topology {
					discoveredLayer3Networks = append(discoveredLayer3Networks, &network)
				}
				if network.TopologyType() == ovntypes.Layer2Topology {
					discoveredLayer2Networks = append(discoveredLayer2Networks, &network)
				}
			}
		case apitypes.PrimaryUserDefinedNetworks:
			namespaceSelector, err := metav1.LabelSelectorAsSelector(&selector.PrimaryUserDefinedNetworkSelector.NamespaceSelector)
			if err != nil {
				return nil, nil, err
			}
			namespaces, err := c.namespaceLister.List(namespaceSelector)
			if err != nil {
				return nil, nil, err
			}
			for _, ns := range namespaces {
				namespacePrimaryNetwork, err := c.networkManager.GetActiveNetworkForNamespace(ns.Name)
				if err != nil {
					return nil, nil, err
				}
				if namespacePrimaryNetwork.IsDefault() || !namespacePrimaryNetwork.IsPrimaryNetwork() {
					continue
				}
				if namespacePrimaryNetwork.TopologyType() == ovntypes.Layer3Topology {
					discoveredLayer3Networks = append(discoveredLayer3Networks, &namespacePrimaryNetwork)
				}
				if namespacePrimaryNetwork.TopologyType() == ovntypes.Layer2Topology {
					discoveredLayer2Networks = append(discoveredLayer2Networks, &namespacePrimaryNetwork)
				}
			}
		default:
			return nil, nil, fmt.Errorf("%w: unsupported network selection type %s", errConfig, selector.NetworkSelectionType)
		}
	}
	return discoveredLayer3Networks, discoveredLayer2Networks, nil
}

func computeNodeLayer3NetworkOwner(nodeName string, networkID int) string {
	return fmt.Sprintf("layer3_%d_%s", networkID, nodeName)
}

func (c *Controller) allocateSubnets(layer3Networks []*util.NetInfo, layer2Networks []*util.NetInfo, connectSubnets networkconnectv1.DualStackCIDRs,
	cncName string) (map[string][]*net.IPNet, error) {
	p2pAllocator, err := c.fetchP2PSubnetAllocator(connectSubnets, cncName)
	if err != nil {
		return nil, err
	}
	allocations := make(map[string][]*net.IPNet)
	if len(layer3Networks) > 0 {
		nodes, err := c.nodeLister.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, network := range layer3Networks {
			netInfo := *network
			networkID := netInfo.GetNetworkID()
			if networkID == ovntypes.NoNetworkID {
				return nil, fmt.Errorf("network id is invalid for network %s", netInfo.GetNetworkName())
			}

			for _, node := range nodes {
				owner := computeNodeLayer3NetworkOwner(node.Name, networkID)
				subnets, err := p2pAllocator.AllocateNetworks(owner)
				if err != nil {
					return nil, err
				}
				klog.V(5).Infof("Allocated subnets %v for %s (network: %s)", subnets, owner, netInfo.GetNetworkName())
				allocations[owner] = subnets
			}
		}
	}
	return allocations, nil
}

func (c *Controller) fetchP2PSubnetAllocator(connectSubnets networkconnectv1.DualStackCIDRs, cncName string) (node.SubnetAllocator, error) {
	c.p2pSubnetAllocatorLock.Lock()
	defer c.p2pSubnetAllocatorLock.Unlock()
	p2pAllocator, ok := c.p2pSubnetAllocator[cncName]
	if !ok {
		p2pAllocator = node.NewSubnetAllocator()
		c.p2pSubnetAllocator[cncName] = p2pAllocator
		for _, connectSubnet := range connectSubnets {
			_, netCIDR, err := net.ParseCIDR(string(connectSubnet))
			if err != nil {
				return nil, err
			}
			if utilnet.IsIPv4CIDR(netCIDR) && config.IPv4Mode {
				if err := p2pAllocator.AddNetworkRange(netCIDR, p2pIPV4SubnetMask); err != nil {
					return nil, err
				}
				klog.V(5).Infof("Added network range %s to cluster network connect %s subnet allocator", netCIDR, cncName)
			}
			if utilnet.IsIPv6CIDR(netCIDR) && config.IPv6Mode {
				if err := p2pAllocator.AddNetworkRange(netCIDR, p2pIPV6SubnetMask); err != nil {
					return nil, err
				}
				klog.V(5).Infof("Added network range %s to cluster network connect %s subnet allocator", netCIDR, cncName)
			}
		}
	}
	return p2pAllocator, nil
}

type p2pSubnetAllocator struct {
	connectSubnets networkconnectv1.DualStackCIDRs
}

func (a *p2pSubnetAllocator) AllocateSubnet(network *util.NetInfo) (*util.NetInfo, error) {
	return nil, nil
}
