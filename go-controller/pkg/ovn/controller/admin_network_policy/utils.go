package adminnetworkpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	libovsdbops "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdb/ops"
	libovsdbutil "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdb/util"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/nbdb"
	addressset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/address_set"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	anpapi "sigs.k8s.io/network-policy-api/apis/v1alpha1"
)

var ErrorANPPriorityUnsupported = errors.New("OVNK only supports priority ranges 0-99")
var ErrorANPWithDuplicatePriority = errors.New("exists with the same priority")

// getAdminNetworkPolicyPGName will return the hashed name and provided anp name as the port group name
func getAdminNetworkPolicyPGName(name string, isBanp bool) (hashedPGName, readablePGName string) {
	readablePortGroupName := fmt.Sprintf("ANP:%s", name)
	if isBanp {
		readablePortGroupName = fmt.Sprintf("BANP:%s", name)
	}
	return util.HashForOVN(readablePortGroupName), readablePortGroupName
}

// getANPRuleACLDbIDs will return the dbObjectIDs for a given rule's ACLs
func getANPRuleACLDbIDs(name, gressPrefix, gressIndex, protocol, controller string, isBanp bool) *libovsdbops.DbObjectIDs {
	idType := libovsdbops.ACLAdminNetworkPolicy
	if isBanp {
		idType = libovsdbops.ACLBaselineAdminNetworkPolicy
	}
	return libovsdbops.NewDbObjectIDs(idType, controller, map[libovsdbops.ExternalIDKey]string{
		libovsdbops.ObjectNameKey:      name,
		libovsdbops.PolicyDirectionKey: gressPrefix,
		// gressidx is the unique id for address set within given objectName and gressPrefix
		libovsdbops.GressIdxKey: gressIndex,
		// protocol key
		libovsdbops.PortPolicyProtocolKey: protocol,
	})
}

// GetACLActionForANPRule returns the corresponding OVN ACL action for a given ANP rule action
func GetACLActionForANPRule(action anpapi.AdminNetworkPolicyRuleAction) string {
	var ovnACLAction string
	switch action {
	case anpapi.AdminNetworkPolicyRuleActionAllow:
		ovnACLAction = nbdb.ACLActionAllowRelated
	case anpapi.AdminNetworkPolicyRuleActionDeny:
		ovnACLAction = nbdb.ACLActionDrop
	case anpapi.AdminNetworkPolicyRuleActionPass:
		ovnACLAction = nbdb.ACLActionPass
	default:
		panic(fmt.Sprintf("Failed to build ANP ACL: unknown acl action %s", action))
	}
	return ovnACLAction
}

// GetACLActionForBANPRule returns the corresponding OVN ACL action for a given BANP rule action
func GetACLActionForBANPRule(action anpapi.BaselineAdminNetworkPolicyRuleAction) string {
	var ovnACLAction string
	switch action {
	case anpapi.BaselineAdminNetworkPolicyRuleActionAllow:
		ovnACLAction = nbdb.ACLActionAllowRelated
	case anpapi.BaselineAdminNetworkPolicyRuleActionDeny:
		ovnACLAction = nbdb.ACLActionDrop
	default:
		panic(fmt.Sprintf("Failed to build BANP ACL: unknown acl action %s", action))
	}
	return ovnACLAction
}

// GetANPPeerAddrSetDbIDs will return the dbObjectIDs for a given rule's address-set
func GetANPPeerAddrSetDbIDs(name, gressPrefix, gressIndex, controller string, isBanp bool) *libovsdbops.DbObjectIDs {
	idType := libovsdbops.AddressSetAdminNetworkPolicy
	if isBanp {
		idType = libovsdbops.AddressSetBaselineAdminNetworkPolicy
	}
	return libovsdbops.NewDbObjectIDs(idType, controller, map[libovsdbops.ExternalIDKey]string{
		libovsdbops.ObjectNameKey:      name,
		libovsdbops.PolicyDirectionKey: gressPrefix,
		// gressidx is the unique id for address set within given objectName and gressPrefix
		libovsdbops.GressIdxKey: gressIndex,
	})
}

func getDirectionFromGressPrefix(gressPrefix string) string {
	if gressPrefix == string(libovsdbutil.ACLIngress) {
		return "src"
	}
	return "dst"
}

// constructMatchFromAddressSet returns the L3Match for an ACL constructed from a gressRule
func constructMatchFromAddressSet(gressPrefix string, addrSetIndex *libovsdbops.DbObjectIDs) string {
	hashedAddressSetNameIPv4, hashedAddressSetNameIPv6 := addressset.GetHashNamesForAS(addrSetIndex)
	var match string
	direction := getDirectionFromGressPrefix(gressPrefix)

	switch {
	case config.IPv4Mode && config.IPv6Mode:
		match = fmt.Sprintf("(ip4.%s == $%s || ip6.%s == $%s)", direction, hashedAddressSetNameIPv4, direction, hashedAddressSetNameIPv6)
	case config.IPv4Mode:
		match = fmt.Sprintf("(ip4.%s == $%s)", direction, hashedAddressSetNameIPv4)
	case config.IPv6Mode:
		match = fmt.Sprintf("(ip6.%s == $%s)", direction, hashedAddressSetNameIPv6)
	}

	return fmt.Sprintf("(%s)", match)
}

// getACLLoggingLevelsForANP takes the ANP's annotations:
// if the "k8s.ovn.org/acl-logging" is set, it parses it
// if parsed values are correct, then it returns those aclLogLevels
// if annotation is not set or parsed values are incorrect/invalid, then it returns empty aclLogLevels which implies logging is disabled
func getACLLoggingLevelsForANP(annotations map[string]string) (*libovsdbutil.ACLLoggingLevels, error) {
	aclLogLevels := &libovsdbutil.ACLLoggingLevels{
		Allow: "", Deny: "", Pass: "",
	}
	annotation, ok := annotations[util.AclLoggingAnnotation]
	if !ok {
		return aclLogLevels, nil
	}
	// If the annotation is "" or "{}", use empty strings. Otherwise, parse the annotation.
	if annotation != "" && annotation != "{}" {
		err := json.Unmarshal([]byte(annotation), aclLogLevels)
		if err != nil {
			// Disable Allow, Deny, Pass logging to ensure idempotency.
			aclLogLevels.Allow = ""
			aclLogLevels.Deny = ""
			aclLogLevels.Pass = ""
			return aclLogLevels, fmt.Errorf("could not unmarshal ANP ACL annotation '%s', disabling logging, err: %q",
				annotation, err)
		}
	}

	// Valid log levels are the various preestablished levels or the empty string.
	validLogLevels := sets.NewString(nbdb.ACLSeverityAlert, nbdb.ACLSeverityWarning, nbdb.ACLSeverityNotice,
		nbdb.ACLSeverityInfo, nbdb.ACLSeverityDebug, "")
	var errors []error
	// Ensure value parsed is valid
	// Set Deny logging.
	if !validLogLevels.Has(aclLogLevels.Deny) {
		errors = append(errors, fmt.Errorf("disabling deny logging due to an invalid deny annotation. "+
			"%q is not a valid log severity", aclLogLevels.Deny))
		aclLogLevels.Deny = ""
	}

	// Set Allow logging.
	if !validLogLevels.Has(aclLogLevels.Allow) {
		errors = append(errors, fmt.Errorf("disabling allow logging due to an invalid allow annotation. "+
			"%q is not a valid log severity", aclLogLevels.Allow))
		aclLogLevels.Allow = ""
	}

	// Set Pass logging.
	if !validLogLevels.Has(aclLogLevels.Pass) {
		errors = append(errors, fmt.Errorf("disabling pass logging due to an invalid pass annotation. "+
			"%q is not a valid log severity", aclLogLevels.Pass))
		aclLogLevels.Pass = ""
	}
	return aclLogLevels, apierrors.NewAggregate(errors)
}

/* NAMED PORTS LOGIC */
type L3L4NamedPortMatch struct{
	ipFamily string
	L3podIP  string
	L4podPort string
}

func (c *Controller) getProtocolPodIPPortMap(rulePeers []*adminNetworkPolicyPeer, namedPorts sets.Set[string]) (map[string]*[]L3L4NamedPortMatch, error) {
	// {k:protocol(string), v:L3L4NamedPortMatch(struct)}
	protocolPodIPPortMap := make(map[string]*[]L3L4NamedPortMatch)
	for _, peer := range rulePeers {
		for namespace, podCache := range peer.namespaces {
			podNamespaceLister := c.anpPodLister.Pods(namespace)
			for _, podName := range podCache.UnsortedList() {
				pod, err := podNamespaceLister.Get(podName)
				if err != nil {
					return nil, err
				}
				podIPs, _ := util.GetPodIPsOfNetwork(pod, &util.DefaultNetInfo{})
				if err != nil {
					if errors.Is(err, util.ErrNoPodIPFound) {
						// we ignore podIPsNotFound error here because onANPPodUpdate
						// will take care of this (if the annotation/LSP add happens later);
						// no need to add nil podIPs to slice...move on to next item in the loop
						klog.Infof("SURYA %v/%v", namespace, podName)
						continue
					}
					// we won't hit this condition TBH because the only error that GetPodIPsOfNetwork returns is podIPsNotFound
					return nil, err
				}
				for _, container := range pod.Spec.Containers {
					for _, port := range container.Ports {
						if port.Name == "" {
							continue
						}
						if namedPorts.Has(port.Name) {
							ppp, ok := protocolPodIPPortMap[string(port.Protocol)]
							if !ok {
								ppp = &[]L3L4NamedPortMatch{}
								protocolPodIPPortMap[libovsdbutil.ConvertK8sProtocolToOVNProtocol(port.Protocol)] = ppp
							}
							for _, podIP := range podIPs {
								newPPP := L3L4NamedPortMatch{
									L4podPort: fmt.Sprintf("%d", port.ContainerPort),
									L3podIP: podIP.String(),
								}
								family := ""
								if utilnet.IsIPv4(podIP) {
									family = "ip4"
								} else {
									family = "ip6"
								}
								newPPP.ipFamily = family
								*ppp = append(*ppp, newPPP)
							}
						}
					}
				}
			}
		}
	}
	return protocolPodIPPortMap, nil
}

func (c *Controller) getL3L4MatchesFromNamedPorts(rulePeers []*adminNetworkPolicyPeer, namedPorts sets.Set[string], gressPrefix string) (map[string]string, error) {
	// {k:protocol(string), v:l3l4Match(string)}
	l3l4NamedPortsMatches := make(map[string]string)
	egressProtoPortsMap, err := c.getProtocolPodIPPortMap(rulePeers, namedPorts)
	if err != nil {
		return nil, err
	}
	direction := getDirectionFromGressPrefix(gressPrefix)
	for protocol, egressPortPodIPs := range egressProtoPortsMap {
		var l3l4match string
		for i, egressPortPodIP := range *egressPortPodIPs {
			l3l4match = l3l4match + fmt.Sprintf("(%s.%s == %s && %s.dst == %s)",
				egressPortPodIP.ipFamily, direction, egressPortPodIP.L3podIP, protocol, egressPortPodIP.L4podPort)
			if i != len(*egressPortPodIPs)-1 {
				l3l4match = l3l4match + "||"
			}
		}
		klog.Infof("SURYA %s", fmt.Sprintf("%s && %s", protocol, l3l4match))
		l3l4NamedPortsMatches[protocol] = fmt.Sprintf("%s && %s", protocol, l3l4match)
	}
	return l3l4NamedPortsMatches, nil
}
