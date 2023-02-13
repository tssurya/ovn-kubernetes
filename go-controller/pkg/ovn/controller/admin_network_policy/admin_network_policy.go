package admin_network_policy

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	libovsdbclient "github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/config"
	anpapi "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/networkpolicyapi/v1alpha1"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdbops"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/nbdb"
	addressset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/address_set"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// NOTE: Iteration v1 of ANP will only support upto 200 ANPs
// We will use priority range from 32000 to 12000 ACLs (both inclusive)
// In order to support more in the future, we will need to fix this in OVS
// See https://bugzilla.redhat.com/show_bug.cgi?id=2175752 for more details.
const (
	ANPFlowStartPriority    = 32000
	ANPMaxPriorityPerObject = 100
	ANPIngressPrefix        = "ANPIngress"
	ANPEgressPrefix         = "ANPEgress"
)

// TODO: Double check how empty selector means all labels match works
type adminNetworkPolicySubject struct {
	namespaceSelector labels.Selector
	podSelector       labels.Selector
	// map of pods matching the provided namespaceSelector and podSelector
	pods *sync.Map // pod's name -> port UUIDS
}

// TODO: Implement sameLabels & notSameLabels
type adminNetworkPolicyPeer struct {
	namespaceSelector labels.Selector
	podSelector       labels.Selector
	// map of pods matching the provided namespaceSelector and podSelector
	pods *sync.Map // pod's name -> ips in the addrSet
}

type adminNetworkPolicyPort struct {
	protocol string
	port     int32 // will store startPort if its a range
	endPort  int32
}

type gressRule struct {
	name        string
	priority    int32 // determined based on order in the list
	gressPrefix string
	action      anpapi.AdminNetworkPolicyRuleAction
	peers       []*adminNetworkPolicyPeer
	ports       []*adminNetworkPolicyPort
	addrSet     addressset.AddressSet
}

type adminNetworkPolicy struct {
	sync.RWMutex
	name         string
	priority     int32
	subject      *adminNetworkPolicySubject
	ingressRules []*gressRule
	egressRules  []*gressRule
	stale        bool
}

// shallow copies the ANP object provided.
func cloneANP(raw *anpapi.AdminNetworkPolicy) (*adminNetworkPolicy, error) {
	anp := &adminNetworkPolicy{
		name:         raw.Name,
		priority:     (ANPFlowStartPriority - raw.Spec.Priority*ANPMaxPriorityPerObject),
		ingressRules: make([]*gressRule, 0),
		egressRules:  make([]*gressRule, 0),
	}

	subjectNamespaceSelector, err := metav1.LabelSelectorAsSelector(raw.Spec.Subject.Namespaces)
	if err != nil {
		return nil, err
	}
	if !subjectNamespaceSelector.Empty() {
		anp.subject = &adminNetworkPolicySubject{
			namespaceSelector: subjectNamespaceSelector,
			podSelector:       labels.Everything(), // it means match all pods within the provided namespaces
		}
	} else {
		// anp.Spec.Subject.Namespaces is not set; anp.Spec.Subject.Pods is set instead
		subjectNamespaceSelector, err = metav1.LabelSelectorAsSelector(&raw.Spec.Subject.Pods.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		subjectPodSelector, err := metav1.LabelSelectorAsSelector(&raw.Spec.Subject.Pods.PodSelector)
		if err != nil {
			return nil, err
		}
		anp.subject = &adminNetworkPolicySubject{
			namespaceSelector: subjectNamespaceSelector,
			podSelector:       subjectPodSelector,
		}
	}

	addErrors := errors.New("")
	for i, rule := range raw.Spec.Ingress {
		anpRule, err := cloneANPIngressRule(rule, anp.priority-int32(i))
		if err != nil {
			addErrors = errors.Wrapf(addErrors, "error: cannot create anp ingress Rule %d in ANP %s - %w",
				i, raw.Name, err)
			continue
		}
		anp.ingressRules = append(anp.ingressRules, anpRule)
	}
	for i, rule := range raw.Spec.Egress {
		anpRule, err := cloneANPEgressRule(rule, raw.Spec.Priority-int32(i))
		if err != nil {
			addErrors = errors.Wrapf(addErrors, "error: cannot create anp egress Rule %d in ANP %s - %w",
				i, raw.Name, err)
			continue
		}
		anp.egressRules = append(anp.egressRules, anpRule)
	}

	if addErrors.Error() == "" {
		addErrors = nil
	}
	return anp, addErrors
}

func getPortProtocol(raw anpapi.AdminNetworkPolicyPort) string {
	var protocol string
	switch raw.PortNumber.Protocol {
	case v1.ProtocolTCP:
		protocol = "tcp"
	case v1.ProtocolSCTP:
		protocol = "sctp"
	case v1.ProtocolUDP:
		protocol = "udp"
	}
	return protocol
}

func cloneANPPort(raw anpapi.AdminNetworkPolicyPort) *adminNetworkPolicyPort {
	var anpPort *adminNetworkPolicyPort
	if raw.PortNumber != nil {
		anpPort.protocol = getPortProtocol(raw)
		anpPort.port = raw.PortNumber.Port
	} else if raw.NamedPort != nil {
		// TODO: Add support for this
	} else {
		anpPort.protocol = getPortProtocol(raw)
		anpPort.port = raw.PortRange.Start
		anpPort.port = raw.PortRange.End
	}
	return anpPort
}

func cloneANPPeer(raw anpapi.AdminNetworkPolicyPeer) (*adminNetworkPolicyPeer, error) {
	var anpPeer *adminNetworkPolicyPeer
	peerNamespaceSelector, err := metav1.LabelSelectorAsSelector(raw.Namespaces.NamespaceSelector)
	if err != nil {
		return nil, err
	}
	if !peerNamespaceSelector.Empty() {
		anpPeer = &adminNetworkPolicyPeer{
			namespaceSelector: peerNamespaceSelector,
			podSelector:       labels.Everything(), // it means match all pods within the provided namespaces
		}
	} else {
		peerNamespaceSelector, err = metav1.LabelSelectorAsSelector(raw.Pods.Namespaces.NamespaceSelector)
		if err != nil {
			return nil, err
		}
		peerPodSelector, err := metav1.LabelSelectorAsSelector(&raw.Pods.PodSelector)
		if err != nil {
			return nil, err
		}
		anpPeer = &adminNetworkPolicyPeer{
			namespaceSelector: peerNamespaceSelector,
			podSelector:       peerPodSelector,
		}
	}
	return anpPeer, nil
}

// shallow copies the ANPRule objects provided.
func cloneANPIngressRule(raw anpapi.AdminNetworkPolicyIngressRule, priority int32) (*gressRule, error) {
	anpRule := &gressRule{
		name:        raw.Name,
		priority:    priority,
		action:      raw.Action,
		gressPrefix: ANPIngressPrefix,
		peers:       make([]*adminNetworkPolicyPeer, 0),
		ports:       make([]*adminNetworkPolicyPort, 0),
	}
	for _, peer := range raw.From {
		anpPeer, err := cloneANPPeer(peer)
		if err != nil {
			return nil, err
		}
		anpRule.peers = append(anpRule.peers, anpPeer)
	}
	if raw.Ports != nil {
		for _, port := range *raw.Ports {
			anpPort := cloneANPPort(port)
			anpRule.ports = append(anpRule.ports, anpPort)
		}
	}

	return anpRule, nil
}

// shallow copies the ANPRule objects provided.
func cloneANPEgressRule(raw anpapi.AdminNetworkPolicyEgressRule, priority int32) (*gressRule, error) {
	anpRule := &gressRule{
		name:        raw.Name,
		priority:    priority,
		action:      raw.Action,
		gressPrefix: ANPEgressPrefix,
		peers:       make([]*adminNetworkPolicyPeer, 0),
		ports:       make([]*adminNetworkPolicyPort, 0),
	}
	for _, peer := range raw.To {
		anpPeer, err := cloneANPPeer(peer)
		if err != nil {
			return nil, err
		}
		anpRule.peers = append(anpRule.peers, anpPeer)
	}
	for _, port := range *raw.Ports {
		anpPort := cloneANPPort(port)
		anpRule.ports = append(anpRule.ports, anpPort)
	}

	return anpRule, nil
}

func (c *Controller) syncAdminNetworkPolicy(key string) error {
	startTime := time.Now()
	_, anpName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	klog.Infof("Processing sync for Admin Network Policy %s", anpName)

	defer func() {
		klog.V(4).Infof("Finished syncing Admin Network Policy %s : %v", anpName, time.Since(startTime))
	}()

	anp, err := c.anpLister.Get(anpName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("SURYA %s", anp)
	// TODO; Make it more efficient if possible by comparing with cache if we do need to change anything or not.
	// delete existing setup if any and recreate
	err = c.clearAdminNetworkPolicy(anp)
	if err != nil {
		return err
	}

	if anp == nil { // it was deleted no need to process further
		return nil
	}

	return c.setAdminNetworkPolicy(anp)
}

func (c *Controller) setAdminNetworkPolicy(anpObj *anpapi.AdminNetworkPolicy) error {
	anp, err := cloneANP(anpObj)
	if err != nil {
		return err
	}

	anp.Lock()
	defer anp.Unlock()
	anp.stale = true // until we finish processing successfully

	// If more than one ANP is created at the same priority behaviour
	// is undefined in k8s upsteam.
	// If more than one ANP is created at the same priority
	// OVNK will only create the first incoming ANP, rest of them will not be created.
	// there should not be an item in the cache for the given priority
	// as we first attempt to delete before create.
	if _, loaded := c.anpCache.LoadOrStore(anp.priority, anp); loaded {
		return fmt.Errorf("error attempting to add ANP %s with priority %d when, "+
			"it already has an ANP with the same priority", anpObj.Name, anpObj.Spec.Priority)
	}

	portGroupName, readableGroupName := getAdminNetworkPolicyPGName(anp.name, anp.priority)
	ops := []ovsdb.Operation{}
	acls, err := c.buildANPACLs(anp, portGroupName)
	if err != nil {
		return fmt.Errorf("unable to build acls for anp %s: %w", anp.name, err)
	}
	ops, err = libovsdbops.CreateOrUpdateACLsOps(c.nbClient, ops, acls...)
	if err != nil {
		return fmt.Errorf("failed to create ACL ops: %v", err)
	}
	podsCache, ports, err := c.getPortsOfSubject(anp.subject.namespaceSelector, anp.subject.podSelector)
	if err != nil {
		return fmt.Errorf("unable to fetch ports for anp %s: %w", anp.name, err)
	}
	anp.subject.pods = podsCache
	pg := libovsdbops.BuildPortGroup(portGroupName, readableGroupName, ports, acls)
	ops, err = libovsdbops.CreateOrUpdatePortGroupsOps(c.nbClient, ops, pg)
	if err != nil {
		return fmt.Errorf("failed to create ops to add port to a port group: %v", err)
	}
	_, err = libovsdbops.TransactAndCheck(c.nbClient, ops)
	if err != nil {
		return fmt.Errorf("failed to run ovsdb txn to add ports to port group: %v", err)
	}
	anp.stale = false // we can mark it as "ready" now
	return nil
}

func getAdminNetworkPolicyPGName(name string, priority int32) (hashedPGName, readablePGName string) {
	pgName := fmt.Sprintf("%s_%d", name, priority)
	return util.HashForOVN(pgName), pgName
}

func (c *Controller) buildANPACLs(anp *adminNetworkPolicy, pgName string) ([]*nbdb.ACL, error) {
	acls := []*nbdb.ACL{}
	for _, ingressRule := range anp.ingressRules {
		acl, err := c.convertRuleToACL(ingressRule, pgName, anp.name)
		if err != nil {
			return nil, err
		}
		acls = append(acls, acl...)
	}
	for _, egressRule := range anp.egressRules {
		acl, err := c.convertRuleToACL(egressRule, pgName, anp.name)
		if err != nil {
			return nil, err
		}
		acls = append(acls, acl...)
	}

	return acls, nil
}

func (c *Controller) convertRuleToACL(rule *gressRule, pgName, anpName string) ([]*nbdb.ACL, error) {
	// create address-set
	// TODO: revisit this to see if its better to do one address-set per peer if that is more perf efficient.
	var err error
	rule.addrSet, err = c.createASForPeers(rule, anpName)
	if err != nil {
		return nil, fmt.Errorf("unable to create address set for "+
			" rule %s with priority %d: %w", rule.name, rule.priority, err)
	}
	// create match based on direction and address-set
	l3Match := constructMatchFromAddressSet(rule)
	// create match based on rule type (ingress/egress) and port-group
	var lportMatch, match, direction string
	var options, extIDs map[string]string
	if rule.gressPrefix == ANPIngressPrefix {
		lportMatch = fmt.Sprintf("(outport == @%s)", pgName)
		direction = nbdb.ACLDirectionToLport
	} else {
		lportMatch = fmt.Sprintf("(inport == @%s)", pgName)
		direction = nbdb.ACLDirectionFromLport
		options = map[string]string{
			"apply-after-lb": "true",
		}
	}
	acls := []*nbdb.ACL{}
	if len(rule.ports) == 0 {
		match = fmt.Sprintf("%s && %s", l3Match, lportMatch)
		acl := libovsdbops.BuildACL(
			// TODO: revisit the logic behind ACL names and extIDs; indexer
			getANPGressPolicyACLName(anpName, rule.gressPrefix, rule.priority),
			direction,
			int(rule.priority),
			match,
			getACLActionForANP(rule.action),
			types.OvnACLLoggingMeter,
			nbdb.ACLSeverityDebug, // TODO: FIX THIS LATER
			false,                 // TODO: FIX THIS LATER
			extIDs,                // TODO: FIX THIS LATER
			options,
		)
		acls = append(acls, acl)
	} else {
		for _, port := range rule.ports {
			l4Match := constructMatchFromPorts(port)
			match = fmt.Sprintf("%s && %s && %s", l3Match, l4Match, lportMatch)
			acl := libovsdbops.BuildACL(
				// TODO: revisit the logic behind ACL names and extIDs; indexer
				getANPGressPolicyACLName(anpName, rule.gressPrefix, rule.priority),
				direction,
				int(rule.priority),
				match,
				getACLActionForANP(rule.action),
				types.OvnACLLoggingMeter,
				nbdb.ACLSeverityDebug, // TODO: FIX THIS LATER
				false,                 // TODO: FIX THIS LATER
				extIDs,                // TODO: FIX THIS LATER
				options,
			)
			acls = append(acls, acl)
		}
	}

	return acls, nil
}

func getACLActionForANP(action anpapi.AdminNetworkPolicyRuleAction) string {
	var ovnACLAction string
	switch action {
	case anpapi.AdminNetworkPolicyRuleActionAllow:
		ovnACLAction = nbdb.ACLActionAllowRelated
	case anpapi.AdminNetworkPolicyRuleActionDeny:
		ovnACLAction = nbdb.ACLActionDrop
	case anpapi.AdminNetworkPolicyRuleActionPass:
		// TODO: FIX THIS LATER WHEN PASS IS IMPLEMENTED IN OVN
		// See https://bugzilla.redhat.com/show_bug.cgi?id=2134138 for details.
		ovnACLAction = nbdb.ACLActionAllow
	default:
		panic(fmt.Sprintf("Failed to build ANP ACL: unknown acl action %s", action))
	}
	return ovnACLAction
}

func (c *Controller) createASForPeers(rule *gressRule, anpName string) (addressset.AddressSet, error) {
	var addrSet addressset.AddressSet
	var err error
	podsIps := []net.IP{}
	for _, peer := range rule.peers {
		podsCache := sync.Map{}
		// TODO: Double check how empty selector means all labels match works
		namespaces, err := c.anpNamespaceLister.List(peer.namespaceSelector)
		if err != nil {
			return nil, err
		}
		for _, namespace := range namespaces {
			podNamespaceLister := c.anpPodLister.Pods(namespace.Name)
			pods, err := podNamespaceLister.List(peer.podSelector)
			if err != nil {
				return nil, err
			}
			for _, pod := range pods {
				// we don't handle HostNetworked or completed pods
				if !util.PodWantsHostNetwork(pod) && !util.PodCompleted(pod) {
					podIPs, err := util.GetPodIPsOfNetwork(pod, &util.DefaultNetInfo{})
					if err != nil {
						return nil, err
					}
					podsCache.Store(pod.Name, podIPs)
					podsIps = append(podsIps, podIPs...)
				}
			}
		}
		peer.pods = &podsCache
	}
	asIndex := getANPPeerAddrSetDbIDs(fmt.Sprintf("%s/%s", anpName, rule.gressPrefix), fmt.Sprintf("%d", rule.priority), controllerName)
	addrSet, err = c.addressSetFactory.EnsureAddressSet(asIndex)
	if err != nil {
		return nil, err
	}
	err = addrSet.SetIPs(podsIps)
	if err != nil {
		return nil, err
	}
	return addrSet, nil
}

func getANPPeerAddrSetDbIDs(name, priority, controller string) *libovsdbops.DbObjectIDs {
	return libovsdbops.NewDbObjectIDs(libovsdbops.AddressSetAdminNetworkPolicy, controller, map[libovsdbops.ExternalIDKey]string{
		libovsdbops.ObjectNameKey: name,
		// priority is the unique id for address set within given gressPrefix
		libovsdbops.PriorityKey: priority,
	})
}

func constructMatchFromAddressSet(rule *gressRule) string {
	hashedAddressSetNameIPv4, hashedAddressSetNameIPv6 := rule.addrSet.GetASHashNames()
	var direction, match string
	if rule.gressPrefix == ANPIngressPrefix {
		direction = "src"
	} else {
		direction = "dst"
	}

	switch {
	case config.IPv4Mode && config.IPv6Mode:
		match = fmt.Sprintf("(ip4.%s == $%s || ip6.%s == $%s)", direction, hashedAddressSetNameIPv4, direction, hashedAddressSetNameIPv6)
	case config.IPv4Mode:
		match = fmt.Sprintf("ip4.%s == $%s", direction, hashedAddressSetNameIPv4)
	case config.IPv6Mode:
		match = fmt.Sprintf("ip6.%s == $%s", direction, hashedAddressSetNameIPv6)
	}

	return fmt.Sprintf("(%s)", match)
}

func constructMatchFromPorts(port *adminNetworkPolicyPort) string {
	if port.endPort != 0 && port.endPort != port.port {
		return fmt.Sprintf("(%s && %d<=%s.dst<=%d)", port.protocol, port.port, port.protocol, port.endPort)

	} else if port.port != 0 {
		return fmt.Sprintf("(%s && %s.dst==%d)", port.protocol, port.protocol, port.port)
	}
	return fmt.Sprintf("(%s)", port.protocol)
}

func getANPGressPolicyACLName(anpName, gressPrefix string, priority int32) string {
	substrings := []string{anpName, gressPrefix, fmt.Sprintf("%d", priority)}
	return strings.Join(substrings, "_")
}

func (c *Controller) getPortsOfSubject(namespaceSelector, podSelector labels.Selector) (*sync.Map, []*nbdb.LogicalSwitchPort, error) {
	ports := []*nbdb.LogicalSwitchPort{}
	namespaces, err := c.anpNamespaceLister.List(namespaceSelector)
	if err != nil {
		return nil, nil, err
	}
	podsCache := sync.Map{}
	for _, namespace := range namespaces {
		podNamespaceLister := c.anpPodLister.Pods(namespace.Name)
		pods, err := podNamespaceLister.List(podSelector)
		if err != nil {
			return nil, nil, err
		}
		for _, pod := range pods {
			if util.PodWantsHostNetwork(pod) || util.PodCompleted(pod) {
				continue
			}
			logicalPortName := util.GetLogicalPortName(pod.Namespace, pod.Name)
			lsp := &nbdb.LogicalSwitchPort{Name: logicalPortName}
			lsp, err = libovsdbops.GetLogicalSwitchPort(c.nbClient, lsp)
			if err != nil {
				if errors.Is(err, libovsdbclient.ErrNotFound) { // TODO: Double check this logic
					continue
				}
				return nil, nil, fmt.Errorf("error retrieving logical switch port with Name %s "+
					" from libovsdb cache: %w", logicalPortName, err)
			}
			ports = append(ports, lsp)
			podsCache.Store(pod.Name, lsp.UUID)
		}
	}

	return &podsCache, ports, nil
}

func (c *Controller) clearAdminNetworkPolicy(anpObj *anpapi.AdminNetworkPolicy) error {
	obj, loaded := c.anpCache.Load(anpObj.Spec.Priority)
	if !loaded {
		// there is no existing ANP configured with this priority, nothing to clean
		klog.V(4).Infof("ANP %s for priority %d not found in cache", anpObj.Name, anpObj.Spec.Priority)
		return nil
	}

	anp := obj.(*adminNetworkPolicy)
	anp.Lock()
	defer anp.Unlock()

	if anpObj.Name != anp.name {
		// there is no existing ANP configured with this name at the given priority, nothing to clean
		klog.V(4).Infof("ANP %s for priority %d not found in cache", anpObj.Name, anpObj.Spec.Priority)
		return nil
	}
	// clear NBDB objects for the given ANP (PG, ACLs on that PG, AddrSets used by the ACLs)
	var err error
	// remove PG for Subject (ACLs will get cleaned up automatically)
	portGroupName, readableGroupName := getAdminNetworkPolicyPGName(anp.name, anp.priority)
	// no need to batch this with address-set deletes since this itself will contain a bunch of ACLs that need to be deleted which is heavy enough.
	err = libovsdbops.DeletePortGroups(c.nbClient, portGroupName)
	if err != nil {
		return fmt.Errorf("unable to delete PG %s for ANP %s: %w", readableGroupName, anp.name, err)
	}
	ops := []ovsdb.Operation{}
	// remove address-sets that were created for the peers of each rule
	for _, ingressRule := range anp.ingressRules {
		ops, err = c.clearASForPeers(anp.name, ingressRule.gressPrefix, ingressRule.priority, ops)
		if err != nil {
			return fmt.Errorf("failed to form delete ops for ANP %s's ingress rule with priority %d: %w", anp.name, ingressRule.priority, err)
		}
	}
	for _, egressRule := range anp.egressRules {
		ops, err = c.clearASForPeers(anp.name, egressRule.gressPrefix, egressRule.priority, ops)
		if err != nil {
			return fmt.Errorf("failed to form delete ops for ANP %s's egress rule with priority %d: %w", anp.name, egressRule.priority, err)
		}
	}
	_, err = libovsdbops.TransactAndCheck(c.nbClient, ops)
	if err != nil {
		return fmt.Errorf("failed to run ovsdb txn to delete address-sets for ANP %s with priority %d: %w", anp.name, anp.priority, err)
	}
	// we can delete the object from the cache now.
	// we also mark it as stale to prevent pod processing if RLock
	// acquired after removal from cache.
	c.anpCache.Delete(anpObj.Spec.Priority)
	anp.stale = true

	return nil
}

func (c *Controller) clearASForPeers(anpName, gressPrefix string, priority int32, ops []ovsdb.Operation) ([]ovsdb.Operation, error) {
	var err error
	predicateIDs := libovsdbops.NewDbObjectIDs(libovsdbops.AddressSetAdminNetworkPolicy, controllerName,
		map[libovsdbops.ExternalIDKey]string{
			libovsdbops.ObjectNameKey: fmt.Sprintf("%s/%s", anpName, gressPrefix),
		})
	asPredicate := libovsdbops.GetPredicate[*nbdb.AddressSet](predicateIDs, nil)
	ops, err = libovsdbops.DeleteAddressSetsWithPredicateOps(c.nbClient, ops, asPredicate)
	if err != nil {
		return nil, fmt.Errorf("failed to contruct ops for ANP address sets, err: %v", err)
	}
	return ops, nil
}
