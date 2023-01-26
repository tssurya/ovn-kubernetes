package clustermanager

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"syscall"
	"time"

	ocpcloudnetworkapi "github.com/openshift/api/cloudnetwork/v1"
	egressipv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressip/v1"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/metrics"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/ovn/healthcheck"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	kapi "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
)

const (
	egressIPReachabilityCheckInterval = 5 * time.Second
)

func getEgressIPAllocationTotalCount(allocator allocator) float64 {
	count := 0
	allocator.Lock()
	defer allocator.Unlock()
	for _, eNode := range allocator.cache {
		count += len(eNode.allocations)
	}
	return float64(count)
}

type egressIPHealthcheckClientAllocator struct{}

func (hccAlloc *egressIPHealthcheckClientAllocator) allocate(nodeName string) healthcheck.EgressIPHealthClient {
	return healthcheck.NewEgressIPHealthClient(nodeName)
}

func isReachableViaGRPC(mgmtIPs []net.IP, healthClient healthcheck.EgressIPHealthClient, healthCheckPort, totalTimeout int) bool {
	dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Duration(totalTimeout)*time.Second)
	defer dialCancel()

	if !healthClient.IsConnected() {
		// gRPC session is not up. Attempt to connect and if that suceeds, we will declare node as reacheable.
		return healthClient.Connect(dialCtx, mgmtIPs, healthCheckPort)
	}

	// gRPC session is already established. Send a probe, which will succeed, or close the session.
	return healthClient.Probe(dialCtx)
}

type egressIPDialer interface {
	dial(ip net.IP, timeout time.Duration) bool
}

type egressIPDial struct{}

var dialer egressIPDialer = &egressIPDial{}

type healthcheckClientAllocator interface {
	allocate(nodeName string) healthcheck.EgressIPHealthClient
}

// Blantant copy from: https://github.com/openshift/sdn/blob/master/pkg/network/common/egressip.go#L499-L505
// Ping a node and return whether or not we think it is online. We do this by trying to
// open a TCP connection to the "discard" service (port 9); if the node is offline, the
// attempt will either time out with no response, or else return "no route to host" (and
// we will return false). If the node is online then we presumably will get a "connection
// refused" error; but the code below assumes that anything other than timeout or "no
// route" indicates that the node is online.
func (e *egressIPDial) dial(ip net.IP, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), "9"), timeout)
	if conn != nil {
		conn.Close()
	}
	if opErr, ok := err.(*net.OpError); ok {
		if opErr.Timeout() {
			return false
		}
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok && sysErr.Err == syscall.EHOSTUNREACH {
			return false
		}
	}
	return true
}


var hccAllocator healthcheckClientAllocator = &egressIPHealthcheckClientAllocator{}

// egressNode is a cache helper used for egress IP assignment, representing an egress node
type egressNode struct {
	egressIPConfig     *util.ParsedNodeEgressIPConfiguration
	mgmtIPs            []net.IP
	allocations        map[string]string
	healthClient       healthcheck.EgressIPHealthClient
	isReady            bool
	isReachable        bool
	isEgressAssignable bool
	name               string
}

func (e *egressNode) getAllocationCountForEgressIP(name string) (count int) {
	for _, egressIPName := range e.allocations {
		if egressIPName == name {
			count++
		}
	}
	return
}

// isAnyClusterNodeIP verifies that the IP is not any node IP.
func (cm *ClusterManager) isAnyClusterNodeIP(ip net.IP) *egressNode {
	for _, eNode := range cm.eIPC.allocator.cache {
		if ip.Equal(eNode.egressIPConfig.V6.IP) || ip.Equal(eNode.egressIPConfig.V4.IP) {
			return eNode
		}
	}
	return nil
}

type EgressIPPatchStatus struct {
	Op    string                    `json:"op"`
	Path  string                    `json:"path"`
	Value egressipv1.EgressIPStatus `json:"value"`
}

// patchReplaceEgressIPStatus performs a replace patch operation of the egress
// IP status by replacing the status with the provided value. This allows us to
// update only the status field, without overwriting any other. This is
// important because processing egress IPs can take a while (when running on a
// public cloud and in the worst case), hence we don't want to perform a full
// object update which risks resetting the EgressIP object's fields to the state
// they had when we started processing the change.
func (cm *ClusterManager) patchReplaceEgressIPStatus(name string, statusItems []egressipv1.EgressIPStatusItem) error {
	klog.Infof("Patching status on EgressIP %s: %v", name, statusItems)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		t := []EgressIPPatchStatus{
			{
				Op:   "replace",
				Path: "/status",
				Value: egressipv1.EgressIPStatus{
					Items: statusItems,
				},
			},
		}
		op, err := json.Marshal(&t)
		if err != nil {
			return fmt.Errorf("error serializing status patch operation: %+v, err: %v", statusItems, err)
		}
		return cm.kube.PatchEgressIP(name, op)
	})
}

type allocator struct {
	*sync.Mutex
	// A cache used for egress IP assignments containing data for all cluster nodes
	// used for egress IP assignments
	cache map[string]*egressNode
}

// podAssignmentState keeps track of which egressIP object is serving
// the related pod.
// NOTE: At a given time only one object will be configured. This is
// transparent to the user
type podAssignmentState struct {
	// the name of the egressIP object that is currently serving this pod
	egressIPName string
	// the list of egressIPs within the above egressIP object that are serving this pod
	egressStatuses map[*egressipv1.EgressIPStatusItem]string
	// list of other egressIP object names that also match this pod but are on standby
	standbyEgressIPNames sets.String
}

type cloudPrivateIPConfigOp struct {
	toAdd    string
	toDelete string
}

// executeCloudPrivateIPConfigChange computes a diff between what needs to be
// assigned/removed and executes the object modification afterwards.
// Specifically: if one egress IP is moved from nodeA to nodeB, we actually care
// about an update on the CloudPrivateIPConfig object represented by that egress
// IP, cloudPrivateIPConfigOp is a helper used to determine that sort of
// operations from toAssign/toRemove
func (cm *ClusterManager) executeCloudPrivateIPConfigChange(egressIPName string, toAssign, toRemove []egressipv1.EgressIPStatusItem) error {
	cm.eIPC.pendingCloudPrivateIPConfigsMutex.Lock()
	defer cm.eIPC.pendingCloudPrivateIPConfigsMutex.Unlock()
	ops := make(map[string]*cloudPrivateIPConfigOp, len(toAssign)+len(toRemove))
	for _, assignment := range toAssign {
		ops[assignment.EgressIP] = &cloudPrivateIPConfigOp{
			toAdd: assignment.Node,
		}
	}
	for _, removal := range toRemove {
		if op, exists := ops[removal.EgressIP]; exists {
			op.toDelete = removal.Node
		} else {
			ops[removal.EgressIP] = &cloudPrivateIPConfigOp{
				toDelete: removal.Node,
			}
		}
	}
	// Merge ops into the existing pendingCloudPrivateIPConfigsOps.
	// This allows us to:
	// a) execute only the new ops
	// b) keep track of any pending changes
	if len(ops) > 0 {
		if _, ok := cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName]; !ok {
			// Set all operations for the EgressIP object if none are in the cache currently.
			cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName] = ops
		} else {
			for cloudPrivateIP, op := range ops {
				if _, ok := cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName][cloudPrivateIP]; !ok {
					// If this specific EgressIP object's CloudPrivateIPConfig address currently has no
					// op, simply set it.
					cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName][cloudPrivateIP] = op
				} else {
					// If an existing operation for this CloudPrivateIP exists, then the following logic should
					// apply:
					// If toDelete is currently set: keep the current toDelete. Theoretically, the oldest toDelete
					// is the good one. If toDelete if currently not set, overwrite it with the new value.
					// If toAdd is currently set: overwrite with the new toAdd. Theoretically, the newest toAdd is
					// the good one.
					// Therefore, only replace toAdd over a previously existing op and only replace toDelete if
					// it's unset.
					if op.toAdd != "" {
						cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName][cloudPrivateIP].toAdd = op.toAdd
					}
					if cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName][cloudPrivateIP].toDelete == "" {
						cm.eIPC.pendingCloudPrivateIPConfigsOps[egressIPName][cloudPrivateIP].toDelete = op.toDelete
					}
				}
			}
		}
	}
	return cm.executeCloudPrivateIPConfigOps(egressIPName, ops)
}

// ipStringToCloudPrivateIPConfigName converts the net.IP string representation
// to a CloudPrivateIPConfig compatible name.

// The string representation of the IPv6 address fc00:f853:ccd:e793::54 will be
// represented as: fc00.f853.0ccd.e793.0000.0000.0000.0054

// We thus need to fully expand the IP string and replace every fifth
// character's colon with a dot.
func ipStringToCloudPrivateIPConfigName(ipString string) (name string) {
	ip := net.ParseIP(ipString)
	if ip.To4() != nil {
		return ipString
	}
	dst := make([]byte, hex.EncodedLen(len(ip)))
	hex.Encode(dst, ip)
	for i := 0; i < len(dst); i += 4 {
		if len(dst)-i == 4 {
			name += string(dst[i : i+4])
		} else {
			name += string(dst[i:i+4]) + "."
		}
	}
	return
}

func (cm *ClusterManager) executeCloudPrivateIPConfigOps(egressIPName string, ops map[string]*cloudPrivateIPConfigOp) error {
	for egressIP, op := range ops {
		cloudPrivateIPConfigName := ipStringToCloudPrivateIPConfigName(egressIP)
		cloudPrivateIPConfig, err := cm.watchFactory.GetCloudPrivateIPConfig(cloudPrivateIPConfigName)
		// toAdd and toDelete is non-empty, this indicates an UPDATE for which
		// the object **must** exist, if not: that's an error.
		if op.toAdd != "" && op.toDelete != "" {
			if err != nil {
				return fmt.Errorf("cloud update request failed for CloudPrivateIPConfig: %s, could not get item, err: %v", cloudPrivateIPConfigName, err)
			}
			// Do not update if object is being deleted
			if !cloudPrivateIPConfig.GetDeletionTimestamp().IsZero() {
				return fmt.Errorf("cloud update request failed, CloudPrivateIPConfig: %s is being deleted", cloudPrivateIPConfigName)
			}
			cloudPrivateIPConfig.Spec.Node = op.toAdd
			if _, err := cm.kube.UpdateCloudPrivateIPConfig(cloudPrivateIPConfig); err != nil {
				eIPRef := kapi.ObjectReference{
					Kind: "EgressIP",
					Name: egressIPName,
				}
				cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "CloudUpdateFailed", "egress IP: %s for object EgressIP: %s could not be updated, err: %v", egressIP, egressIPName, err)
				return fmt.Errorf("cloud update request failed for CloudPrivateIPConfig: %s, err: %v", cloudPrivateIPConfigName, err)
			}
			// toAdd is non-empty, this indicates an ADD
			// if the object already exists for the specified node that's a no-op
			// if the object already exists and the request is for a different node, that's an error
		} else if op.toAdd != "" {
			if err == nil {
				if op.toAdd == cloudPrivateIPConfig.Spec.Node {
					klog.Infof("CloudPrivateIPConfig: %s already assigned to node: %s", cloudPrivateIPConfigName, cloudPrivateIPConfig.Spec.Node)
					continue
				}
				return fmt.Errorf("cloud create request failed for CloudPrivateIPConfig: %s, err: item exists", cloudPrivateIPConfigName)
			}
			cloudPrivateIPConfig := ocpcloudnetworkapi.CloudPrivateIPConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudPrivateIPConfigName,
					Annotations: map[string]string{
						util.OVNEgressIPOwnerRefLabel: egressIPName,
					},
				},
				Spec: ocpcloudnetworkapi.CloudPrivateIPConfigSpec{
					Node: op.toAdd,
				},
			}
			if _, err := cm.kube.CreateCloudPrivateIPConfig(&cloudPrivateIPConfig); err != nil {
				eIPRef := kapi.ObjectReference{
					Kind: "EgressIP",
					Name: egressIPName,
				}
				cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "CloudAssignmentFailed", "egress IP: %s for object EgressIP: %s could not be created, err: %v", egressIP, egressIPName, err)
				return fmt.Errorf("cloud add request failed for CloudPrivateIPConfig: %s, err: %v", cloudPrivateIPConfigName, err)
			}
			// toDelete is non-empty, this indicates a DELETE - if the object does not exist, log an Info message and continue with the next op.
			// The reason for why we are not throwing an error here is that desired state (deleted) == isState (object not found).
			// If for whatever reason we have a pending toDelete op for a deleted object, then this op should simply be silently ignored.
			// Any other error, return an error to trigger a retry.
		} else if op.toDelete != "" {
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.Infof("Cloud deletion request failed for CloudPrivateIPConfig: %s, item already deleted, err: %v", cloudPrivateIPConfigName, err)
					continue
				} else {
					return fmt.Errorf("cloud deletion request failed for CloudPrivateIPConfig: %s, could not get item, err: %v", cloudPrivateIPConfigName, err)
				}
			}
			if err := cm.kube.DeleteCloudPrivateIPConfig(cloudPrivateIPConfigName); err != nil {
				eIPRef := kapi.ObjectReference{
					Kind: "EgressIP",
					Name: egressIPName,
				}
				cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "CloudDeletionFailed", "egress IP: %s for object EgressIP: %s could not be deleted, err: %v", egressIP, egressIPName, err)
				return fmt.Errorf("cloud deletion request failed for CloudPrivateIPConfig: %s, err: %v", cloudPrivateIPConfigName, err)
			}
		}
	}
	return nil
}

type egressIPController struct {
	// egressIPAssignmentMutex is used to ensure a safe updates between
	// concurrent go-routines which could be modifying the egress IP status
	// assignment simultaneously. Currently WatchEgressNodes and WatchEgressIP
	// run two separate go-routines which do this.
	egressIPAssignmentMutex *sync.Mutex
	// podAssignmentMutex is used to ensure safe access to podAssignment.
	// Currently WatchEgressIP, WatchEgressNamespace and WatchEgressPod could
	// all access that map simultaneously, hence why this guard is needed.
	podAssignmentMutex *sync.Mutex
	// podAssignment is a cache used for keeping track of which egressIP status
	// has been setup for each pod. The key is defined by getPodKey
	podAssignment map[string]*podAssignmentState
	// pendingCloudPrivateIPConfigsMutex is used to ensure synchronized access
	// to pendingCloudPrivateIPConfigsOps which is accessed by the egress IP and
	// cloudPrivateIPConfig go-routines
	pendingCloudPrivateIPConfigsMutex *sync.Mutex
	// pendingCloudPrivateIPConfigsOps is a cache of pending
	// CloudPrivateIPConfig changes that we are waiting on an answer for. Items
	// in this map are only ever removed once the op is fully finished and we've
	// been notified of this. That means:
	// - On add operations we only delete once we've seen that the
	// CloudPrivateIPConfig is fully added.
	// - On delete: when it's fully deleted.
	// - On update: once we finish processing the add - which comes after the
	// delete.
	pendingCloudPrivateIPConfigsOps map[string]map[string]*cloudPrivateIPConfigOp
	// allocator is a cache of egress IP centric data needed to when both route
	// health-checking and tracking allocations made
	allocator allocator
	// watchFactory watching k8s objects
	watchFactory *factory.WatchFactory
	// EgressIP Node reachability total timeout configuration
	egressIPTotalTimeout int
	// reachability check interval
	reachabilityCheckInterval time.Duration
	// EgressIP Node reachability gRPC port (0 means it should use dial instead)
	egressIPNodeHealthCheckPort int
}

type egressIPNodeStatus struct {
	Node string
	Name string
}

// getSortedEgressData returns a sorted slice of all egressNodes based on the
// amount of allocations found in the cache
func (cm *ClusterManager) getSortedEgressData() ([]*egressNode, map[string]egressIPNodeStatus) {
	assignableNodes := []*egressNode{}
	allAllocations := make(map[string]egressIPNodeStatus)
	for _, eNode := range cm.eIPC.allocator.cache {
		if eNode.isEgressAssignable && eNode.isReady && eNode.isReachable {
			assignableNodes = append(assignableNodes, eNode)
		}
		for ip, eipName := range eNode.allocations {
			allAllocations[ip] = egressIPNodeStatus{Node: eNode.name, Name: eipName}
		}
	}
	sort.Slice(assignableNodes, func(i, j int) bool {
		return len(assignableNodes[i].allocations) < len(assignableNodes[j].allocations)
	})
	return assignableNodes, allAllocations
}

// setupNodeForEgress sets up default logical router policy for every node and
// initiates the allocator cache for the node in question, if the node has the
// necessary annotation.
func (cm *ClusterManager) setupNodeForEgress(node *v1.Node) error {
	// let's pray local-master has already setup the necessary routes for this node
	if err := cm.initEgressIPAllocator(node); err != nil {
		klog.V(5).Infof("Egress node initialization error: %v", err)
	}
	return nil
}

func (cm *ClusterManager) setNodeEgressAssignable(nodeName string, isAssignable bool) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	if eNode, exists := cm.eIPC.allocator.cache[nodeName]; exists {
		eNode.isEgressAssignable = isAssignable
		// if the node is not assignable/ready/reachable anymore we need to
		// empty all of it's allocations from our cache since we'll clear all
		// assignments from this node later on, because of this.
		if !isAssignable {
			eNode.allocations = make(map[string]string)
		}
	}
}

func (cm *ClusterManager) isEgressNodeReady(egressNode *kapi.Node) bool {
	for _, condition := range egressNode.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

func isReachableLegacy(node string, mgmtIPs []net.IP, totalTimeout int) bool {
	var retryTimeOut, initialRetryTimeOut time.Duration

	numMgmtIPs := len(mgmtIPs)
	if numMgmtIPs == 0 {
		return false
	}

	switch totalTimeout {
	// Check if we need to do node reachability check
	case 0:
		return true
	case 1:
		// Using time duration for initial retry with 700/numIPs msec and retry of 100/numIPs msec
		// to ensure total wait time will be in range with the configured value including a sleep of 100msec between attempts.
		initialRetryTimeOut = time.Duration(700/numMgmtIPs) * time.Millisecond
		retryTimeOut = time.Duration(100/numMgmtIPs) * time.Millisecond
	default:
		// Using time duration for initial retry with 900/numIPs msec
		// to ensure total wait time will be in range with the configured value including a sleep of 100msec between attempts.
		initialRetryTimeOut = time.Duration(900/numMgmtIPs) * time.Millisecond
		retryTimeOut = initialRetryTimeOut
	}

	timeout := initialRetryTimeOut
	endTime := time.Now().Add(time.Second * time.Duration(totalTimeout))
	for time.Now().Before(endTime) {
		for _, ip := range mgmtIPs {
			if dialer.dial(ip, timeout) {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
		timeout = retryTimeOut
	}
	klog.Errorf("Failed reachability check for %s", node)
	return false
}

// checkEgressNodesReachability continuously checks if all nodes used for egress
// IP assignment are reachable, and updates the nodes following the result. This
// is important because egress IP is based upon routing traffic to these nodes,
// and if they aren't reachable we shouldn't be using them for egress IP.
func (cm *ClusterManager) checkEgressNodesReachability() {
	timer := time.NewTicker(cm.eIPC.reachabilityCheckInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			checkEgressNodesReachabilityIterate(cm)
		case <-cm.stopChan:
			klog.V(5).Infof("Stop channel got triggered: will stop checkEgressNodesReachability")
			return
		}
	}
}

func checkEgressNodesReachabilityIterate(cm *ClusterManager) {
	reAddOrDelete := map[string]bool{}
	cm.eIPC.allocator.Lock()
	for _, eNode := range cm.eIPC.allocator.cache {
		if eNode.isEgressAssignable && eNode.isReady {
			wasReachable := eNode.isReachable
			isReachable := cm.isReachable(eNode.name, eNode.mgmtIPs, eNode.healthClient)
			if wasReachable && !isReachable {
				reAddOrDelete[eNode.name] = true
			} else if !wasReachable && isReachable {
				reAddOrDelete[eNode.name] = false
			}
			eNode.isReachable = isReachable
		} else {
			// End connection (if there is one). This is important because
			// it accounts for cases where node is not labelled with
			// egress-assignable, so connection is no longer needed. Calling
			// this on a already disconnected node is expected to be cheap.
			eNode.healthClient.Disconnect()
		}
	}
	cm.eIPC.allocator.Unlock()
	for nodeName, shouldDelete := range reAddOrDelete {
		if shouldDelete {
			metrics.RecordEgressIPUnreachableNode()
			klog.Warningf("Node: %s is detected as unreachable, deleting it from egress assignment", nodeName)
			if err := cm.deleteEgressNode(nodeName); err != nil {
				klog.Errorf("Node: %s is detected as unreachable, but could not re-assign egress IPs, err: %v", nodeName, err)
			}
		} else {
			klog.Infof("Node: %s is detected as reachable and ready again, adding it to egress assignment", nodeName)
			if err := cm.addEgressNode(nodeName); err != nil {
				klog.Errorf("Node: %s is detected as reachable and ready again, but could not re-assign egress IPs, err: %v", nodeName, err)
			}
		}
	}
}

func (cm *ClusterManager) isReachable(nodeName string, mgmtIPs []net.IP, healthClient healthcheck.EgressIPHealthClient) bool {
	// Check if we need to do node reachability check
	if cm.eIPC.egressIPTotalTimeout == 0 {
		return true
	}

	if cm.eIPC.egressIPNodeHealthCheckPort == 0 {
		return isReachableLegacy(nodeName, mgmtIPs, cm.eIPC.egressIPTotalTimeout)
	}
	return isReachableViaGRPC(mgmtIPs, healthClient, cm.eIPC.egressIPNodeHealthCheckPort, cm.eIPC.egressIPTotalTimeout)
}

func (cm *ClusterManager) isEgressNodeReachable(egressNode *kapi.Node) bool {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	if eNode, exists := cm.eIPC.allocator.cache[egressNode.Name]; exists {
		return eNode.isReachable || cm.isReachable(eNode.name, eNode.mgmtIPs, eNode.healthClient)
	}
	return false
}

func (cm *ClusterManager) setNodeEgressReady(nodeName string, isReady bool) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	if eNode, exists := cm.eIPC.allocator.cache[nodeName]; exists {
		eNode.isReady = isReady
		// see setNodeEgressAssignable
		if !isReady {
			eNode.allocations = make(map[string]string)
		}
	}
}

func (cm *ClusterManager) setNodeEgressReachable(nodeName string, isReachable bool) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	if eNode, exists := cm.eIPC.allocator.cache[nodeName]; exists {
		eNode.isReachable = isReachable
		// see setNodeEgressAssignable
		if !isReachable {
			eNode.allocations = make(map[string]string)
		}
	}
}

func (cm *ClusterManager) addEgressNode(nodeName string) error {
	var errors []error
	klog.V(5).Infof("Egress node: %s about to be initialized", nodeName)

	// If a node has been labelled for egress IP we need to check if there are any
	// egress IPs which are missing an assignment. If there are, we need to send a
	// synthetic update since reconcileEgressIP will then try to assign those IPs to
	// this node (if possible)
	egressIPs, err := cm.kube.GetEgressIPs()
	if err != nil {
		return fmt.Errorf("unable to list EgressIPs, err: %v", err)
	}
	for _, egressIP := range egressIPs.Items {
		if len(egressIP.Spec.EgressIPs) != len(egressIP.Status.Items) {
			// Send a "synthetic update" on all egress IPs which are not fully
			// assigned, the reconciliation loop for WatchEgressIP will try to
			// assign stuff to this new node. The workqueue's delta FIFO
			// implementation will not trigger a watch event for updates on
			// objects which have no semantic difference, hence: call the
			// reconciliation function directly.
			if err := cm.reconcileEgressIP(nil, &egressIP); err != nil {
				errors = append(errors, fmt.Errorf("synthetic update for EgressIP: %s failed, err: %v", egressIP.Name, err))
			}
		}
	}

	if len(errors) > 0 {
		return utilerrors.NewAggregate(errors)
	}
	return nil
}

func (cm *ClusterManager) deleteEgressNode(nodeName string) error {
	var errorAggregate []error
	klog.V(5).Infof("Egress node: %s about to be removed", nodeName)
	// Since the node has been labelled as "not usable" for egress IP
	// assignments we need to find all egress IPs which have an assignment to
	// it, and move them elsewhere.
	egressIPs, err := cm.kube.GetEgressIPs()
	if err != nil {
		return fmt.Errorf("unable to list EgressIPs, err: %v", err)
	}
	for _, egressIP := range egressIPs.Items {
		for _, status := range egressIP.Status.Items {
			if status.Node == nodeName {
				// Send a "synthetic update" on all egress IPs which have an
				// assignment to this node. The reconciliation loop for
				// WatchEgressIP will see that the current assignment status to
				// this node is invalid and try to re-assign elsewhere. The
				// workqueue's delta FIFO implementation will not trigger a
				// watch event for updates on objects which have no semantic
				// difference, hence: call the reconciliation function directly.
				if err := cm.reconcileEgressIP(nil, &egressIP); err != nil {
					errorAggregate = append(errorAggregate, fmt.Errorf("re-assignment for EgressIP: %s failed, unable to update object, err: %v", egressIP.Name, err))
				}
				break
			}
		}
	}
	if len(errorAggregate) > 0 {
		return utilerrors.NewAggregate(errorAggregate)
	}
	return nil
}

func (cm *ClusterManager) initEgressIPAllocator(node *kapi.Node) (err error) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	if _, exists := cm.eIPC.allocator.cache[node.Name]; !exists {
		var parsedEgressIPConfig *util.ParsedNodeEgressIPConfiguration
		if util.PlatformTypeIsEgressIPCloudProvider() {
			parsedEgressIPConfig, err = util.ParseCloudEgressIPConfig(node)
			if err != nil {
				return fmt.Errorf("unable to use cloud node for egress assignment, err: %v", err)
			}
		} else {
			parsedEgressIPConfig, err = util.ParseNodePrimaryIfAddr(node)
			if err != nil {
				return fmt.Errorf("unable to use node for egress assignment, err: %v", err)
			}
		}
		nodeSubnets, err := util.ParseNodeHostSubnetAnnotation(node, types.DefaultNetworkName)
		if err != nil {
			return fmt.Errorf("failed to parse node %s subnets annotation %v", node.Name, err)
		}
		mgmtIPs := make([]net.IP, len(nodeSubnets))
		for i, subnet := range nodeSubnets {
			mgmtIPs[i] = util.GetNodeManagementIfAddr(subnet).IP
		}
		cm.eIPC.allocator.cache[node.Name] = &egressNode{
			name:           node.Name,
			egressIPConfig: parsedEgressIPConfig,
			mgmtIPs:        mgmtIPs,
			allocations:    make(map[string]string),
			healthClient:   hccAllocator.allocate(node.Name),
		}
	}
	return nil
}

// deleteAllocatorEgressIPAssignments deletes the allocation as to keep the
// cache state correct, also see addAllocatorEgressIPAssignments
func (cm *ClusterManager) deleteAllocatorEgressIPAssignments(statusAssignments []egressipv1.EgressIPStatusItem) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	for _, status := range statusAssignments {
		if eNode, exists := cm.eIPC.allocator.cache[status.Node]; exists {
			delete(eNode.allocations, status.EgressIP)
		}
	}
}

// deleteAllocatorEgressIPAssignmentIfExists deletes egressIP config from node allocations map
// if the entry is available and returns assigned node name, otherwise returns empty string.
func (cm *ClusterManager) deleteAllocatorEgressIPAssignmentIfExists(name, egressIP string) string {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	for nodeName, eNode := range cm.eIPC.allocator.cache {
		if egressIPName, exists := eNode.allocations[egressIP]; exists && egressIPName == name {
			delete(eNode.allocations, egressIP)
			return nodeName
		}
	}
	return ""
}

// addAllocatorEgressIPAssignments adds the allocation to the cache, so that
// they are tracked during the life-cycle of ovnkube-master
func (cm *ClusterManager) addAllocatorEgressIPAssignments(name string, statusAssignments []egressipv1.EgressIPStatusItem) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	for _, status := range statusAssignments {
		if eNode, exists := cm.eIPC.allocator.cache[status.Node]; exists {
			eNode.allocations[status.EgressIP] = name
		}
	}
}

func (cm *ClusterManager) reconcileEgressIP(old, new *egressipv1.EgressIP) (err error) {
	// Lock the assignment, this is needed because this function can end up
	// being called from WatchEgressNodes and WatchEgressIP, i.e: two different
	// go-routines and we need to make sure the assignment is safe.
	cm.eIPC.egressIPAssignmentMutex.Lock()
	defer cm.eIPC.egressIPAssignmentMutex.Unlock()

	// Initialize an empty name which is filled depending on the operation
	// (ADD/UPDATE/DELETE) we are performing. This is done as to be able to
	// delete the NB DB set up correctly when searching the DB based on the
	// name.
	name := ""

	// Initialize a status which will be used to compare against
	// new.spec.egressIPs and decide on what from the status should get deleted
	// or kept.
	status := []egressipv1.EgressIPStatusItem{}

	// Initialize two empty objects as to avoid SIGSEGV. The code should play
	// nicely with empty objects though.
	oldEIP, newEIP := &egressipv1.EgressIP{}, &egressipv1.EgressIP{}

	// Initialize two "nothing" selectors. Nothing selector are semantically
	// opposed to "empty" selectors, i.e: they select and match nothing, while
	// an empty one matches everything. If old/new are nil, and we don't do
	// this: we would have an empty EgressIP object which would result in two
	// empty selectors, matching everything, whereas we would mean the inverse
	/* //MOVE TO LOCAL CONTROLLER
	newNamespaceSelector, _ := metav1.LabelSelectorAsSelector(nil)
	oldNamespaceSelector, _ := metav1.LabelSelectorAsSelector(nil) // MOVE TO LOCAL CONTROLLER */
	// Initialize a sets.String which holds egress IPs that were not fully assigned
	// but are allocated and they are meant to be removed.
	staleEgressIPs := sets.NewString()
	if old != nil {
		oldEIP = old
		_, err = metav1.LabelSelectorAsSelector(&oldEIP.Spec.NamespaceSelector) // COPY TO LOCAL CONTROLLER
		if err != nil {
			return fmt.Errorf("invalid old namespaceSelector, err: %v", err)
		}
		name = oldEIP.Name
		status = oldEIP.Status.Items
		staleEgressIPs.Insert(oldEIP.Spec.EgressIPs...)
	}
	if new != nil {
		newEIP = new
		_, err = metav1.LabelSelectorAsSelector(&newEIP.Spec.NamespaceSelector) // COPY TO LOCAL CONTROLLER
		if err != nil {
			return fmt.Errorf("invalid new namespaceSelector, err: %v", err)
		}
		name = newEIP.Name
		status = newEIP.Status.Items
		if staleEgressIPs.Len() > 0 {
			for _, egressIP := range newEIP.Spec.EgressIPs {
				if staleEgressIPs.Has(egressIP) {
					staleEgressIPs.Delete(egressIP)
				}
			}
		}
	}

	// We do not initialize a nothing selector for the podSelector, because
	// these are allowed to be empty (i.e: matching all pods in a namespace), as
	// supposed to the namespaceSelector
	/// MOVE TO LOCAL CONTROLLER
	/*newPodSelector, err := metav1.LabelSelectorAsSelector(&newEIP.Spec.PodSelector)
	if err != nil {
		return fmt.Errorf("invalid new podSelector, err: %v", err)
	}
	oldPodSelector, err := metav1.LabelSelectorAsSelector(&oldEIP.Spec.PodSelector)
	if err != nil {
		return fmt.Errorf("invalid old podSelector, err: %v", err)
	} // MOVE TO LOCAL CONTROLLER */

	// Validate the spec and use only the valid egress IPs when performing any
	// successive operations, theoretically: the user could specify invalid IP
	// addresses, which would break us.
	validSpecIPs, err := cm.validateEgressIPSpec(name, newEIP.Spec.EgressIPs)
	if err != nil {
		return fmt.Errorf("invalid EgressIP spec, err: %v", err)
	}

	// Validate the status, on restart it could be the case that what might have
	// been assigned when ovnkube-master last ran is not a valid assignment
	// anymore (specifically if ovnkube-master has been crashing for a while).
	// Any invalid status at this point in time needs to be removed and assigned
	// to a valid node.
	validStatus, invalidStatus := cm.validateEgressIPStatus(name, status)
	for status := range validStatus {
		// If the spec has changed and an egress IP has been removed by the
		// user: we need to un-assign that egress IP
		if !validSpecIPs.Has(status.EgressIP) {
			invalidStatus[status] = ""
			delete(validStatus, status)
		}
	}

	invalidStatusLen := len(invalidStatus)
	if invalidStatusLen > 0 {
		metrics.RecordEgressIPRebalance(invalidStatusLen)
	}

	// Add only the diff between what is requested and valid and that which
	// isn't already assigned.
	ipsToAssign := validSpecIPs
	ipsToRemove := sets.NewString()
	statusToAdd := make([]egressipv1.EgressIPStatusItem, 0, len(ipsToAssign))
	statusToKeep := make([]egressipv1.EgressIPStatusItem, 0, len(validStatus))
	for status := range validStatus {
		statusToKeep = append(statusToKeep, *status)
		ipsToAssign.Delete(status.EgressIP)
	}
	statusToRemove := make([]egressipv1.EgressIPStatusItem, 0, invalidStatusLen)
	for status := range invalidStatus {
		statusToRemove = append(statusToRemove, *status)
		ipsToRemove.Insert(status.EgressIP)
	}
	if ipsToRemove.Len() > 0 {
		// The following is added as to ensure that we only add after having
		// successfully removed egress IPs. This case is not very important on
		// bare-metal (since we execute the add after the remove below, and
		// hence have full control of the execution - barring its success), but
		// on a cloud: we don't execute anything below, we wait for the status
		// on the CloudPrivateIPConfig(s) we create to be set before executing
		// anything in the OVN DB. So, we need to make sure that we delete and
		// then add, mainly because if EIP1 is added to nodeX and then EIP2 is
		// removed from nodeX, we might remove the setup made for EIP1. The
		// add/delete ordering of events is not guaranteed on the cloud where we
		// depend on other controllers to execute the work for us however. By
		// comparing the spec to the status and applying the following truth
		// table we can ensure that order of events.

		// case ID    |    Egress IP to add    |    Egress IP to remove    |    ipsToAssign
		// 1          |    e1                  |    e1                     |    e1
		// 2          |    e2                  |    e1                     |    -
		// 3          |    e2                  |    -                      |    e2
		// 4          |    -                   |    e1                     |    -

		// Case 1 handles updates. Case 2 and 3 makes sure we don't add until we
		// successfully delete. Case 4 just shows an example of what would
		// happen if we don't have anything to add
		ipsToAssign = ipsToAssign.Intersection(ipsToRemove)
	}

	if !util.PlatformTypeIsEgressIPCloudProvider() {
		if len(statusToRemove) > 0 {
			// Delete the statusToRemove from the allocator cache. If we don't
			// do this we will occupy assignment positions for the ipsToAssign,
			// even though statusToRemove will be removed afterwards
			cm.deleteAllocatorEgressIPAssignments(statusToRemove)
		}
		if len(ipsToAssign) > 0 {
			statusToAdd = cm.assignEgressIPs(name, ipsToAssign.UnsortedList())
			statusToKeep = append(statusToKeep, statusToAdd...)
		}
		// Add all assignments which are to be kept to the allocator cache,
		// allowing us to track all assignments which have been performed and
		// avoid incorrect future assignments due to a de-synchronized cache.
		cm.addAllocatorEgressIPAssignments(name, statusToKeep)
		// Update the object only on an ADD/UPDATE. If we are processing a
		// DELETE, new will be nil and we should not update the object.
		if len(statusToAdd) > 0 || (len(statusToRemove) > 0 && new != nil) {
			if err := cm.patchReplaceEgressIPStatus(name, statusToKeep); err != nil {
				return err
			}
		}
	} else {
		// Even when running on a public cloud, we must make sure that we unwire EgressIP
		// configuration from OVN *before* we instruct the CloudNetworkConfigController
		// to remove the CloudPrivateIPConfig object from the cloud.
		// CloudPrivateIPConfig objects can be in the "Deleting" state for a long time,
		// waiting for the underlying cloud to finish its action and to report success of the
		// unattach operation. Some clouds such as Azure will remove the IP address nearly
		// immediately, but then they will take a long time (seconds to minutes) to actually report
		// success of the removal operation.
		if len(statusToRemove) > 0 {
			// Delete all assignments that are to be removed from the allocator
			// cache. If we don't do this we will occupy assignment positions for
			// the ipsToAdd, even though statusToRemove will be removed afterwards
			cm.deleteAllocatorEgressIPAssignments(statusToRemove)
		}
		// When egress IP is not fully assigned to a node, then statusToRemove may not
		// have those entries, hence retrieve it from staleEgressIPs for removing
		// the item from cloudprivateipconfig.
		for _, toRemove := range statusToRemove {
			if !staleEgressIPs.Has(toRemove.EgressIP) {
				continue
			}
			staleEgressIPs.Delete(toRemove.EgressIP)
		}
		for staleEgressIP := range staleEgressIPs {
			if nodeName := cm.deleteAllocatorEgressIPAssignmentIfExists(name, staleEgressIP); nodeName != "" {
				statusToRemove = append(statusToRemove,
					egressipv1.EgressIPStatusItem{EgressIP: staleEgressIP, Node: nodeName})
			}
		}
		// If running on a public cloud we should not program OVN just yet for assignment
		// operations. We need confirmation from the cloud-network-config-controller that
		// it can assign the IPs. reconcileCloudPrivateIPConfig will take care of
		// processing the answer from the requests we make here, and update OVN
		// accordingly when we know what the outcome is.
		if len(ipsToAssign) > 0 {
			statusToAdd = cm.assignEgressIPs(name, ipsToAssign.UnsortedList())
			statusToKeep = append(statusToKeep, statusToAdd...)
		}
		// Same as above: Add all assignments which are to be kept to the
		// allocator cache, allowing us to track all assignments which have been
		// performed and avoid incorrect future assignments due to a
		// de-synchronized cache.
		cm.addAllocatorEgressIPAssignments(name, statusToKeep)

		// Execute CloudPrivateIPConfig changes for assignments which need to be
		// added/removed, assignments which don't change do not require any
		// further setup.
		if err := cm.executeCloudPrivateIPConfigChange(name, statusToAdd, statusToRemove); err != nil {
			return err
		}
	}

	// Record the egress IP allocator count
	metrics.RecordEgressIPCount(getEgressIPAllocationTotalCount(cm.eIPC.allocator))

	////////MOVE TO LOCAL CONTROLLER
	// If nothing has changed for what concerns the assignments, then check if
	// the namespaceSelector and podSelector have changed. If they have changed
	// then remove the setup for all pods which matched the old and add
	// everything for all pods which match the new.
	/*if len(ipsToAssign) == 0 &&
		len(statusToRemove) == 0 {
		// Only the namespace selector changed: remove the setup for all pods
		// matching the old and not matching the new, and add setup for the pod
		// matching the new and which didn't match the old.
		if !reflect.DeepEqual(newNamespaceSelector, oldNamespaceSelector) && reflect.DeepEqual(newPodSelector, oldPodSelector) {
			namespaces, err := cm.watchFactory.GetNamespaces()
			if err != nil {
				return err
			}
			for _, namespace := range namespaces {
				namespaceLabels := labels.Set(namespace.Labels)
				if !newNamespaceSelector.Matches(namespaceLabels) && oldNamespaceSelector.Matches(namespaceLabels) {
					if err := oc.deleteNamespaceEgressIPAssignment(name, oldEIP.Status.Items, namespace, oldEIP.Spec.PodSelector); err != nil {
						return err
					}
				}
				if newNamespaceSelector.Matches(namespaceLabels) && !oldNamespaceSelector.Matches(namespaceLabels) {
					if err := oc.addNamespaceEgressIPAssignments(name, newEIP.Status.Items, namespace, newEIP.Spec.PodSelector); err != nil {
						return err
					}
				}
			}
			// Only the pod selector changed: remove the setup for all pods
			// matching the old and not matching the new, and add setup for the pod
			// matching the new and which didn't match the old.
		} else if reflect.DeepEqual(newNamespaceSelector, oldNamespaceSelector) && !reflect.DeepEqual(newPodSelector, oldPodSelector) {
			namespaces, err := oc.watchFactory.GetNamespacesBySelector(newEIP.Spec.NamespaceSelector)
			if err != nil {
				return err
			}
			for _, namespace := range namespaces {
				pods, err := oc.watchFactory.GetPods(namespace.Name)
				if err != nil {
					return err
				}
				for _, pod := range pods {
					podLabels := labels.Set(pod.Labels)
					if !newPodSelector.Matches(podLabels) && oldPodSelector.Matches(podLabels) {
						if err := oc.deletePodEgressIPAssignments(name, oldEIP.Status.Items, pod); err != nil {
							return err
						}
					}
					if util.PodCompleted(pod) {
						continue
					}
					if newPodSelector.Matches(podLabels) && !oldPodSelector.Matches(podLabels) {
						if err := oc.addPodEgressIPAssignmentsWithLock(name, newEIP.Status.Items, pod); err != nil {
							return err
						}
					}
				}
			}
			// Both selectors changed: remove the setup for pods matching the
			// old ones and not matching the new ones, and add setup for all
			// matching the new ones but which didn't match the old ones.
		} else if !reflect.DeepEqual(newNamespaceSelector, oldNamespaceSelector) && !reflect.DeepEqual(newPodSelector, oldPodSelector) {
			namespaces, err := oc.watchFactory.GetNamespaces()
			if err != nil {
				return err
			}
			for _, namespace := range namespaces {
				namespaceLabels := labels.Set(namespace.Labels)
				// If the namespace does not match anymore then there's no
				// reason to look at the pod selector.
				if !newNamespaceSelector.Matches(namespaceLabels) && oldNamespaceSelector.Matches(namespaceLabels) {
					if err := oc.deleteNamespaceEgressIPAssignment(name, oldEIP.Status.Items, namespace, oldEIP.Spec.PodSelector); err != nil {
						return err
					}
				}
				// If the namespace starts matching, look at the pods selector
				// and pods in that namespace and perform the setup for the pods
				// which match the new pod selector or if the podSelector is empty
				// then just perform the setup.
				if newNamespaceSelector.Matches(namespaceLabels) && !oldNamespaceSelector.Matches(namespaceLabels) {
					pods, err := oc.watchFactory.GetPods(namespace.Name)
					if err != nil {
						return err
					}
					for _, pod := range pods {
						podLabels := labels.Set(pod.Labels)
						if newPodSelector.Matches(podLabels) {
							if err := oc.addPodEgressIPAssignmentsWithLock(name, newEIP.Status.Items, pod); err != nil {
								return err
							}
						}
					}
				}
				// If the namespace continues to match, look at the pods
				// selector and pods in that namespace.
				if newNamespaceSelector.Matches(namespaceLabels) && oldNamespaceSelector.Matches(namespaceLabels) {
					pods, err := oc.watchFactory.GetPods(namespace.Name)
					if err != nil {
						return err
					}
					for _, pod := range pods {
						podLabels := labels.Set(pod.Labels)
						if !newPodSelector.Matches(podLabels) && oldPodSelector.Matches(podLabels) {
							if err := oc.deletePodEgressIPAssignments(name, oldEIP.Status.Items, pod); err != nil {
								return err
							}
						}
						if util.PodCompleted(pod) {
							continue
						}
						if newPodSelector.Matches(podLabels) && !oldPodSelector.Matches(podLabels) {
							if err := oc.addPodEgressIPAssignmentsWithLock(name, newEIP.Status.Items, pod); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	} //////MOVE TO LOCAL CONTROLLER */
	return nil
}

// assignEgressIPs is the main assignment algorithm for egress IPs to nodes.
// Specifically we have a couple of hard constraints: a) the subnet of the node
// must be able to host the egress IP b) the egress IP cannot be a node IP c)
// the IP cannot already be assigned and reference by another EgressIP object d)
// no two egress IPs for the same EgressIP object can be assigned to the same
// node e) (for public clouds) the amount of egress IPs assigned to one node
// must respect its assignment capacity. Moreover there is a soft constraint:
// the assignments need to be balanced across all cluster nodes, so that no node
// becomes a bottleneck. The balancing is achieved by sorting the nodes in
// ascending order following their existing amount of allocations, and trying to
// assign the egress IP to the node with the lowest amount of allocations every
// time, this does not guarantee complete balance, but mostly complete.
func (cm *ClusterManager) assignEgressIPs(name string, egressIPs []string) []egressipv1.EgressIPStatusItem {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	assignments := []egressipv1.EgressIPStatusItem{}
	assignableNodes, existingAllocations := cm.getSortedEgressData()
	if len(assignableNodes) == 0 {
		eIPRef := kapi.ObjectReference{
			Kind: "EgressIP",
			Name: name,
		}
		cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "NoMatchingNodeFound", "no assignable nodes for EgressIP: %s, please tag at least one node with label: %s", name, util.GetNodeEgressLabel())
		klog.Errorf("No assignable nodes found for EgressIP: %s and requested IPs: %v", name, egressIPs)
		return assignments
	}
	klog.V(5).Infof("Current assignments are: %+v", existingAllocations)
	for _, egressIP := range egressIPs {
		klog.V(5).Infof("Will attempt assignment for egress IP: %s", egressIP)
		eIPC := net.ParseIP(egressIP)
		if status, exists := existingAllocations[eIPC.String()]; exists {
			// On public clouds we will re-process assignments for the same IP
			// multiple times due to the nature of syncing each individual
			// CloudPrivateIPConfig one at a time. This means that we are
			// expected to end up in this situation multiple times per sync. Ex:
			// Say we an EgressIP is created with IP1, IP2, IP3. We begin by
			// assigning them all the first round. Next we get the
			// CloudPrivateIPConfig confirming the addition of IP1, leading us
			// to re-assign IP2, IP3, but since we've already assigned them
			// we'll end up here. This is not an error. What would be an error
			// is if the user created EIP1 with IP1 and a second EIP2 with IP1
			if name == status.Name {
				// IP is already assigned for this EgressIP object
				assignments = append(assignments, egressipv1.EgressIPStatusItem{
					Node:     status.Node,
					EgressIP: eIPC.String(),
				})
				continue
			} else {
				klog.Errorf("IP: %q for EgressIP: %s is already allocated for EgressIP: %s on %s", egressIP, name, status.Name, status.Node)
				return assignments
			}
		}
		if node := cm.isAnyClusterNodeIP(eIPC); node != nil {
			eIPRef := kapi.ObjectReference{
				Kind: "EgressIP",
				Name: name,
			}
			cm.recorder.Eventf(
				&eIPRef,
				kapi.EventTypeWarning,
				"UnsupportedRequest",
				"Egress IP: %v for object EgressIP: %s is the IP address of node: %s, this is unsupported", eIPC, name, node.name,
			)
			klog.Errorf("Egress IP: %v is the IP address of node: %s", eIPC, node.name)
			return assignments
		}
		for _, eNode := range assignableNodes {
			klog.V(5).Infof("Attempting assignment on egress node: %+v", eNode)
			if eNode.getAllocationCountForEgressIP(name) > 0 {
				klog.V(5).Infof("Node: %s is already in use by another egress IP for this EgressIP: %s, trying another node", eNode.name, name)
				continue
			}
			if eNode.egressIPConfig.Capacity.IP < util.UnlimitedNodeCapacity {
				if eNode.egressIPConfig.Capacity.IP-len(eNode.allocations) <= 0 {
					klog.V(5).Infof("Additional allocation on Node: %s exhausts it's IP capacity, trying another node", eNode.name)
					continue
				}
			}
			if eNode.egressIPConfig.Capacity.IPv4 < util.UnlimitedNodeCapacity && utilnet.IsIPv4(eIPC) {
				if eNode.egressIPConfig.Capacity.IPv4-getIPFamilyAllocationCount(eNode.allocations, false) <= 0 {
					klog.V(5).Infof("Additional allocation on Node: %s exhausts it's IPv4 capacity, trying another node", eNode.name)
					continue
				}
			}
			if eNode.egressIPConfig.Capacity.IPv6 < util.UnlimitedNodeCapacity && utilnet.IsIPv6(eIPC) {
				if eNode.egressIPConfig.Capacity.IPv6-getIPFamilyAllocationCount(eNode.allocations, true) <= 0 {
					klog.V(5).Infof("Additional allocation on Node: %s exhausts it's IPv6 capacity, trying another node", eNode.name)
					continue
				}
			}
			if (eNode.egressIPConfig.V6.Net != nil && eNode.egressIPConfig.V6.Net.Contains(eIPC)) ||
				(eNode.egressIPConfig.V4.Net != nil && eNode.egressIPConfig.V4.Net.Contains(eIPC)) {
				assignments = append(assignments, egressipv1.EgressIPStatusItem{
					Node:     eNode.name,
					EgressIP: eIPC.String(),
				})
				klog.Infof("Successful assignment of egress IP: %s on node: %+v", egressIP, eNode)
				eNode.allocations[eIPC.String()] = name
				break
			}
		}
	}
	if len(assignments) == 0 {
		eIPRef := kapi.ObjectReference{
			Kind: "EgressIP",
			Name: name,
		}
		cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "NoMatchingNodeFound", "No matching nodes found, which can host any of the egress IPs: %v for object EgressIP: %s", egressIPs, name)
		klog.Errorf("No matching host found for EgressIP: %s", name)
		return assignments
	}
	if len(assignments) < len(egressIPs) {
		eIPRef := kapi.ObjectReference{
			Kind: "EgressIP",
			Name: name,
		}
		cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "UnassignedRequest", "Not all egress IPs for EgressIP: %s could be assigned, please tag more nodes", name)
	}
	return assignments
}

func getIPFamilyAllocationCount(allocations map[string]string, isIPv6 bool) (count int) {
	for allocation := range allocations {
		if utilnet.IsIPv4String(allocation) && !isIPv6 {
			count++
		}
		if utilnet.IsIPv6String(allocation) && isIPv6 {
			count++
		}
	}
	return
}

func (cm *ClusterManager) validateEgressIPSpec(name string, egressIPs []string) (sets.String, error) {
	validatedEgressIPs := sets.NewString()
	for _, egressIP := range egressIPs {
		ip := net.ParseIP(egressIP)
		if ip == nil {
			eIPRef := kapi.ObjectReference{
				Kind: "EgressIP",
				Name: name,
			}
			cm.recorder.Eventf(&eIPRef, kapi.EventTypeWarning, "InvalidEgressIP", "egress IP: %s for object EgressIP: %s is not a valid IP address", egressIP, name)
			return nil, fmt.Errorf("unable to parse provided EgressIP: %s, invalid", egressIP)
		}
		validatedEgressIPs.Insert(ip.String())
	}
	return validatedEgressIPs, nil
}

// validateEgressIPStatus validates if the statuses are valid given what the
// cache knows about all egress nodes. WatchEgressNodes is initialized before
// any other egress IP handler, so the cache should be warm and correct once we
// start going this.
func (cm *ClusterManager) validateEgressIPStatus(name string, items []egressipv1.EgressIPStatusItem) (map[*egressipv1.EgressIPStatusItem]string, map[*egressipv1.EgressIPStatusItem]string) {
	cm.eIPC.allocator.Lock()
	defer cm.eIPC.allocator.Unlock()
	valid, invalid := make(map[*egressipv1.EgressIPStatusItem]string), make(map[*egressipv1.EgressIPStatusItem]string)
	for _, eIPStatus := range items {
		eIPStatus := eIPStatus
		validAssignment := true
		eNode, exists := cm.eIPC.allocator.cache[eIPStatus.Node]
		if !exists {
			klog.Errorf("Allocator error: EgressIP: %s claims to have an allocation on a node which is unassignable for egress IP: %s", name, eIPStatus.Node)
			validAssignment = false
		} else {
			if eNode.getAllocationCountForEgressIP(name) > 1 {
				klog.Errorf("Allocator error: EgressIP: %s claims multiple egress IPs on same node: %s, will attempt rebalancing", name, eIPStatus.Node)
				validAssignment = false
			}
			if !eNode.isEgressAssignable {
				klog.Errorf("Allocator error: EgressIP: %s assigned to node: %s which does not have egress label, will attempt rebalancing", name, eIPStatus.Node)
				validAssignment = false
			}
			if !eNode.isReachable {
				klog.Errorf("Allocator error: EgressIP: %s assigned to node: %s which is not reachable, will attempt rebalancing", name, eIPStatus.Node)
				validAssignment = false
			}
			if !eNode.isReady {
				klog.Errorf("Allocator error: EgressIP: %s assigned to node: %s which is not ready, will attempt rebalancing", name, eIPStatus.Node)
				validAssignment = false
			}
			ip := net.ParseIP(eIPStatus.EgressIP)
			if ip == nil {
				klog.Errorf("Allocator error: EgressIP allocation contains unparsable IP address: %s", eIPStatus.EgressIP)
				validAssignment = false
			}
			if node := cm.isAnyClusterNodeIP(ip); node != nil {
				klog.Errorf("Allocator error: EgressIP allocation: %s is the IP of node: %s ", ip.String(), node.name)
				validAssignment = false
			}
			if utilnet.IsIPv6(ip) && eNode.egressIPConfig.V6.Net != nil {
				if !eNode.egressIPConfig.V6.Net.Contains(ip) {
					klog.Errorf("Allocator error: EgressIP allocation: %s on subnet: %s which cannot host it", ip.String(), eNode.egressIPConfig.V4.Net.String())
					validAssignment = false
				}
			} else if !utilnet.IsIPv6(ip) && eNode.egressIPConfig.V4.Net != nil {
				if !eNode.egressIPConfig.V4.Net.Contains(ip) {
					klog.Errorf("Allocator error: EgressIP allocation: %s on subnet: %s which cannot host it", ip.String(), eNode.egressIPConfig.V4.Net.String())
					validAssignment = false
				}
			} else {
				klog.Errorf("Allocator error: EgressIP allocation on node: %s which does not support its IP protocol version", eIPStatus.Node)
				validAssignment = false
			}
		}
		if validAssignment {
			valid[&eIPStatus] = ""
		} else {
			invalid[&eIPStatus] = ""
		}
	}
	return valid, invalid
}
