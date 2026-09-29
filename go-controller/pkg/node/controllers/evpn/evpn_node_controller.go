// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"reflect"
	"slices"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	utilnet "k8s.io/utils/net"
	"k8s.io/utils/ptr"

	libovsdbclient "github.com/ovn-kubernetes/libovsdb/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nadlisters "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/listers/k8s.cni.cncf.io/v1"

	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/config"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/controller"
	networkconnectv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1"
	networkconnectlisters "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1/apis/listers/clusternetworkconnect/v1"
	apitypes "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/types"
	userdefinednetworkv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	vtepv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vtep/v1"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/kube"
	libovsdbops "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/libovsdb/ops"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/networkmanager"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/node"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/node/netlinkdevicemanager"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/node/routemanager"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/types"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/util"
	utilerrors "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/util/errors"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/vswitchd"
)

// nodeAddressManager provides access to the node's IP addresses
// and allows registering callbacks for address changes.
type nodeAddressManager interface {
	ListAddresses() ([]net.IP, []*net.IPNet)
	AddOnAddressesChangedHandler(handler func())
}

var cudnGVK = userdefinednetworkv1.SchemeGroupVersion.WithKind("ClusterUserDefinedNetwork")

const (
	ovsBridgeInt = "br-int"

	// reconcileNodeAddressChange is a synthetic key enqueued into the VTEP
	// controller workqueue when node addresses change. It triggers
	// reconciliation of unmanaged VTEPs whose annotated IPs are stale.
	reconcileNodeAddressChange = "//node-address-change"

	// reconcileVTEPAnnotationChange is a synthetic key enqueued when the
	// node's VTEP annotation is externally modified or removed. It
	// invalidates the annotation cache and triggers reconciliation to
	// restore the expected state.
	reconcileVTEPAnnotationChange = "//vtep-annotation-change"

	// vtepAnnotationFieldManager identifies this controller as the owner of
	// the VTEP annotation on the node, used to detect external modifications.
	vtepAnnotationFieldManager = "node-vtep-controller"

	// cncParentVRFTableBase is the Linux routing table ID base for CNC parent VRFs.
	// CNC parent VRF table IDs = cncParentVRFTableBase + VID (VID is 1-4094, table IDs 1000001-1004094).
	// This range is safely above regular CUDN VRF tables (RoutingTableIDStart + ifIndex, practically ≤ ~66000)
	// and DPU VRF tables (100000 + networkID, practically ≤ ~110000). It is reserved in config so the VRF
	// manager's stale-VRF repair leaves these NDM-owned VRFs alone (see config.EVPNCNCRoutingTableIDStart).
	cncParentVRFTableBase = config.EVPNCNCRoutingTableIDStart

	// cncSiblingRouteMetric is the priority of the child-VRF routes that steer
	// inter-UDN egress toward the CNC parent VRF. A low, non-zero metric keeps
	// them preferred over any leaked aggregate while still losing to the more
	// specific per-pod routes symmetric IRB imports into the child VRF.
	cncSiblingRouteMetric = 5
)

type Controller struct {
	nodeName       string
	watchFactory   factory.NodeWatchFactory
	kube           kube.Interface
	networkMgr     networkmanager.Interface
	ndm            netlinkdevicemanager.Interface
	ovsClient      libovsdbclient.Client
	addressManager nodeAddressManager
	routeManager   *routemanager.Controller

	// vtepController reconciles VTEP CRs: ensures bridge, VXLAN, SVI, and OVS port
	// lifecycle for each VTEP assigned to this node.
	vtepController controller.Controller
	// nodeEventHandler watches for node annotation changes that should
	// trigger VTEP reconciliation if needed.
	nodeEventHandler cache.ResourceEventHandlerRegistration
	// nadReconciler triggers VTEP reconciliation when NADs referencing a VTEP
	// are added or removed, so VID/VNI mappings and SVIs stay in sync.
	nadReconciler   controller.Reconciler
	nadReconcilerID uint64

	// Cache NAD key -> VTEP name for cleanup when NADs are deleted.
	nadVTEPInfoLock sync.Mutex
	nadVTEPInfo     map[string]string

	// svisByBridge tracks SVI names created per bridge, so we can detect
	// stale SVIs
	svisByBridgeLock sync.Mutex
	svisByBridge     map[string]sets.Set[string]

	// vtepsAnnotation is a controller-local copy of the node's VTEP annotation,
	// used instead of the informer cache to avoid stale reads when multiple
	// VTEPs are reconciled in sequence before the cache catches up.
	vtepsAnnotation map[string]util.VTEPNodeAnnotation

	// podController manages static FDB and neighbor entries for local pods
	// on EVPN networks, enabling ARP/ND suppression and known-unicast forwarding.
	podController controller.Controller
	podLister     corelisters.PodLister
	podNeighLock  sync.Mutex
	podNeighbors  map[string]*neighEntries

	// cncLister, nadLister, and cncController are set when NetworkConnect is enabled.
	// cncController triggers VTEP reconciliation when CNC VID annotation or spec changes.
	// nadLister is used to resolve CNC ClusterUserDefinedNetworks selectors to EVPN networks
	// without needing a VTEP annotation on the CNC object.
	// Note: PrimaryUserDefinedNetworks selectors are not handled here because namespace-scoped
	// UDNs do not support EVPN transport.
	cncLister     networkconnectlisters.ClusterNetworkConnectLister
	nadLister     nadlisters.NetworkAttachmentDefinitionLister
	cncController controller.Controller

	// cncVRFsByBridgeLock guards cncVRFsByBridge and cncVRFDevicesByBridge.
	cncVRFsByBridgeLock sync.Mutex
	// cncVRFsByBridge tracks CNC parent VRF SVI names per bridge for stale cleanup.
	cncVRFsByBridge map[string]sets.Set[string]
	// cncVRFDevicesByBridge tracks CNC parent VRF Linux device names per VTEP bridge.
	// The VRF device is global (not bridge-bound), but is managed alongside the bridge's SVI
	// for consistent lifecycle: when the bridge (VTEP) is deleted all its CNC VRF devices are too.
	cncVRFDevicesByBridge map[string]sets.Set[string]

	// cncSiblingRoutesLock guards cncSiblingRoutesByVTEP.
	cncSiblingRoutesLock sync.Mutex
	// cncSiblingRoutesByVTEP tracks the child-VRF sibling-subnet routes programmed
	// through the route manager per VTEP, so stale routes are removed when a CNC's
	// connected set changes or a network/VTEP is removed.
	cncSiblingRoutesByVTEP map[string]map[siblingRouteKey]siblingRouteEntry

	stopChan chan struct{}
}

// siblingRouteKey identifies a child-VRF sibling route by its destination and
// routing table; it matches the route manager's own (dst, table, priority) key
// space (priority is fixed at cncSiblingRouteMetric for these routes).
type siblingRouteKey struct {
	dst   string
	table int
}

// siblingRouteEntry records a programmed sibling route together with the CNC
// that owns it, so routes for a CNC whose parent VRF is not yet ready can be
// carried forward across reconciles instead of being deleted and re-added.
type siblingRouteEntry struct {
	route netlink.Route
	cnc   string
}

func NewController(nodeName string, wf factory.NodeWatchFactory, kube kube.Interface, ndm netlinkdevicemanager.Interface, networkMgr networkmanager.Interface, ovsClient libovsdbclient.Client, addressManager nodeAddressManager, routeManager *routemanager.Controller) (*Controller, error) {
	if addressManager == nil {
		return nil, fmt.Errorf("EVPN node VTEP controller requires a non-nil node address manager")
	}

	c := &Controller{
		nodeName:               nodeName,
		watchFactory:           wf,
		kube:                   kube,
		networkMgr:             networkMgr,
		ndm:                    ndm,
		ovsClient:              ovsClient,
		addressManager:         addressManager,
		routeManager:           routeManager,
		nadVTEPInfo:            make(map[string]string),
		svisByBridge:           make(map[string]sets.Set[string]),
		cncVRFsByBridge:        make(map[string]sets.Set[string]),
		cncVRFDevicesByBridge:  make(map[string]sets.Set[string]),
		cncSiblingRoutesByVTEP: make(map[string]map[siblingRouteKey]siblingRouteEntry),
		stopChan:               make(chan struct{}),
	}

	vtepInformer := wf.VTEPInformer()
	c.vtepController = controller.NewController("evpn-node-vtep-controller", &controller.ControllerConfig[vtepv1.VTEP]{
		RateLimiter:    workqueue.DefaultTypedControllerRateLimiter[string](),
		Reconcile:      c.reconcile,
		ObjNeedsUpdate: c.vtepNeedsUpdate,
		Threadiness:    1,
		Informer:       vtepInformer.Informer(),
		Lister:         vtepInformer.Lister().List,
	})
	c.nadReconciler = controller.NewReconciler("evpn-nad-reconciler", &controller.ReconcilerConfig{
		RateLimiter: workqueue.DefaultTypedControllerRateLimiter[string](),
		Reconcile:   c.reconcileNAD,
		Threadiness: 1,
		MaxAttempts: controller.InfiniteAttempts,
	})

	podInformer := wf.PodCoreInformer()
	c.podLister = podInformer.Lister()
	c.podNeighbors = make(map[string]*neighEntries)
	c.podController = controller.NewController("evpn-pod-neighbor-controller",
		&controller.ControllerConfig[corev1.Pod]{
			RateLimiter:    workqueue.DefaultTypedControllerRateLimiter[string](),
			Reconcile:      c.reconcilePod,
			ObjNeedsUpdate: c.podNeedsUpdate,
			Threadiness:    1,
			Informer:       podInformer.Informer(),
			Lister:         c.podLister.List,
		})

	var err error
	c.nodeEventHandler, err = wf.NodeCoreInformer().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: c.onNodeUpdate,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to add node event handler: %w", err)
	}

	if util.IsNetworkConnectEnabled() {
		cncInformer := wf.ClusterNetworkConnectInformer()
		c.cncLister = cncInformer.Lister()
		c.nadLister = wf.NADInformer().Lister()
		c.cncController = controller.NewController("evpn-node-cnc-controller", &controller.ControllerConfig[networkconnectv1.ClusterNetworkConnect]{
			RateLimiter: workqueue.DefaultTypedControllerRateLimiter[string](),
			Reconcile: func(_ string) error {
				// The udn-bgp-drop exemptions are node-global and CNC-driven, so
				// reconcile them once here rather than per VTEP.
				if err := c.reconcileCNCIsolationExemptions(); err != nil {
					return err
				}
				c.vtepController.ReconcileAll()
				return nil
			},
			ObjNeedsUpdate: cncNodeNeedsUpdate,
			Threadiness:    1,
			Informer:       cncInformer.Informer(),
			Lister:         c.cncLister.List,
		})
	}

	addressManager.AddOnAddressesChangedHandler(func() {
		c.vtepController.Reconcile(reconcileNodeAddressChange)
	})

	return c, nil
}

func (c *Controller) Start() (err error) {
	klog.Info("Starting EVPN node controller")

	c.nadReconcilerID = c.networkMgr.RegisterNADReconciler(c.nadReconciler)
	defer func() {
		if err != nil {
			c.networkMgr.DeRegisterNADReconciler(c.nadReconcilerID)
			c.nadReconcilerID = 0
		}
	}()
	controllers := []controller.Reconciler{c.vtepController, c.nadReconciler, c.podController}
	if c.cncController != nil {
		controllers = append(controllers, c.cncController)
	}
	return controller.StartWithInitialSync(c.initialSync, controllers...)
}

func (c *Controller) initialSync() error {
	// VTEP informer cache sync is handled by StartWithInitialSync, but we also
	// need the node informer cache synced to read VTEP IPs annotation.
	if !util.WaitForInformerCacheSyncWithTimeout("evpn-node", c.stopChan, c.watchFactory.NodeCoreInformer().Informer().HasSynced) {
		return fmt.Errorf("timed out waiting for node informer cache to sync")
	}

	vteps, err := c.watchFactory.VTEPInformer().Lister().List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VTEPs: %w", err)
	}

	// Pre-populate NDM desired state so it doesn't remove existing devices on startup.
	// Only NDM-managed devices (bridge, VXLANs, SVIs) are handled here.
	// OVS ports are deferred to normal reconciliation after controllers start.
	activeVTEPs := sets.New[string]()
	for _, vtep := range vteps {
		activeVTEPs.Insert(vtep.Name)
		if vtep.Spec.Mode == vtepv1.VTEPModeManaged {
			continue
		}
		vtepIPv4, vtepIPv6, err := c.discoverUnmanagedVTEPIPs(vtep)
		if err != nil {
			klog.Errorf("Failed to get VTEP IPs for %s: %v", vtep.Name, err)
			continue
		}
		if vtepIPv4 == nil && vtepIPv6 == nil {
			continue
		}
		if _, err := c.ensureDevices(vtep, vtepIPv4, vtepIPv6); err != nil {
			klog.Errorf("Failed to pre-populate NDM for VTEP %s: %v", vtep.Name, err)
		}
	}

	// Clean up stale OVS ports from VTEPs that were deleted while the controller was down.
	// NDM handles netlink device cleanup via alias-based ownership, but OVS ports are managed
	// directly through libovsdb and need explicit cleanup here.
	if err := c.cleanupStaleOVSPorts(activeVTEPs); err != nil {
		klog.Errorf("Failed to clean up stale OVS ports: %v", err)
	}

	if err := c.cleanStalePodEntries(); err != nil {
		klog.Errorf("Failed to sync pod neighbors: %v", err)
	}

	return nil
}

func (c *Controller) Stop() {
	klog.Info("Stopping EVPN node controller")

	if c.nodeEventHandler != nil {
		if err := c.watchFactory.NodeCoreInformer().Informer().RemoveEventHandler(c.nodeEventHandler); err != nil {
			klog.Errorf("Failed to remove node event handler: %v", err)
		}
	}

	if c.nadReconcilerID != 0 {
		c.networkMgr.DeRegisterNADReconciler(c.nadReconcilerID)
	}

	controllers := []controller.Reconciler{c.vtepController, c.nadReconciler, c.podController}
	if c.cncController != nil {
		controllers = append(controllers, c.cncController)
	}
	controller.Stop(controllers...)

	close(c.stopChan)
}

func (c *Controller) vtepNeedsUpdate(oldObj, newObj *vtepv1.VTEP) bool {
	if oldObj == nil || newObj == nil {
		return true
	}
	return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
}

func (c *Controller) onNodeUpdate(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		return
	}
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		return
	}
	if newNode.Name != c.nodeName {
		return
	}
	if !util.NodeVTEPsAnnotationChanged(oldNode, newNode) {
		return
	}
	// don't reconcile on our own updates
	if util.IsLastUpdatedByManager(vtepAnnotationFieldManager, newNode.ManagedFields) {
		return
	}

	c.vtepController.Reconcile(reconcileVTEPAnnotationChange)
}

// reconcileNodeAddressChange triggers reconciliation for unmanaged VTEPs whose
// annotated IPs are no longer valid or missing. Called when the node's
// addresses change via the address manager callback.
func (c *Controller) reconcileNodeAddressChange() error {
	vtepsAnnotation, err := c.getVTEPsAnnotation()
	if err != nil {
		return fmt.Errorf("failed to get VTEP annotation for address change reconciliation: %w", err)
	}

	nodeIPs, _ := c.addressManager.ListAddresses()
	nodeIPSet := sets.New(util.StringSlice(nodeIPs)...)

	vteps, err := c.watchFactory.VTEPInformer().Lister().List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VTEPs for address change reconciliation: %w", err)
	}
	for _, vtep := range vteps {
		if vtep.Spec.Mode != vtepv1.VTEPModeUnmanaged {
			continue
		}
		needsIPv4 := util.MatchFirstCIDRStringFamily(false, vtep.Spec.CIDRs) != ""
		needsIPv6 := util.MatchFirstCIDRStringFamily(true, vtep.Spec.CIDRs) != ""
		v4AnnotatedIP := matchFirstIPStringFamily(false, vtepsAnnotation[vtep.Name].IPs)
		v6AnnotatedIP := matchFirstIPStringFamily(true, vtepsAnnotation[vtep.Name].IPs)
		missesIPv4 := needsIPv4 && !nodeIPSet.Has(v4AnnotatedIP)
		missesIPv6 := needsIPv6 && !nodeIPSet.Has(v6AnnotatedIP)
		if missesIPv4 || missesIPv6 {
			klog.V(5).Infof("Node addresses changed on %s, reconciling unmanaged VTEP %s", c.nodeName, vtep.Name)
			c.vtepController.Reconcile(vtep.Name)
		}
	}

	return nil
}

// reconcile ensures all node-local devices for a VTEP are in the desired state:
// 1. Discover/read VTEP IPs
// 2. Ensure bridge, VXLAN tunnels, and VID/VNI mappings via NDM
// 3. Reconcile SVIs and OVS ports for each EVPN-enabled network
// If the VTEP is deleted or unsupported, all its devices are cleaned up.
// The synthetic keys reconcileVTEPAnnotationChange and
// reconcileNodeAddressChange trigger reconciliation of unmanaged VTEPs
// whose annotated IPs are stale or missing.
func (c *Controller) reconcile(key string) error {
	switch key {
	case reconcileVTEPAnnotationChange:
		// node annotation changed, reset the cached annotation to re-read
		c.vtepsAnnotation = nil
		fallthrough
	case reconcileNodeAddressChange:
		return c.reconcileNodeAddressChange()
	}

	vtep, err := c.watchFactory.VTEPInformer().Lister().Get(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("VTEP %s not found, cleaning up", key)
			if err := c.deleteVTEPDevices(key); err != nil {
				return err
			}
			return c.setVTEPAnnotation(key, nil, nil)
		}
		return err
	}
	if vtep.Spec.Mode == vtepv1.VTEPModeManaged {
		klog.Warningf("VTEP %s uses unsupported %s mode, cleaning up", vtep.Name, vtepv1.VTEPModeManaged)
		if err := c.deleteVTEPDevices(key); err != nil {
			return err
		}
		return c.setVTEPAnnotation(key, nil, nil)
	}

	if config.HybridOverlay.Enabled && config.HybridOverlay.VXLANPort == config.DefaultVXLANPort {
		return fmt.Errorf("hybrid overlay is enabled with VXLAN port %d which conflicts with EVPN VXLAN; "+
			"configure a different hybrid-overlay-vxlan-port to avoid the conflict", config.DefaultVXLANPort)
	}

	vtepIPv4, vtepIPv6, err := c.discoverUnmanagedVTEPIPs(vtep)
	if err != nil {
		return fmt.Errorf("failed to discover VTEP %s IPs: %w", vtep.Name, err)
	}
	if vtepIPv4 == nil && vtepIPv6 == nil {
		klog.Infof("VTEP %s IPs not yet available for node %s", vtep.Name, c.nodeName)
		return c.deleteVTEPDevices(key)
	}

	networks, err := c.ensureDevices(vtep, vtepIPv4, vtepIPv6)
	if err != nil {
		return err
	}

	// Refresh the node-global CNC isolation exemptions here as well as from the CNC
	// controller: a network connected by a CNC may only become resolvable (its NetInfo
	// registered) after the CNC event has already been processed, and its readiness
	// drives a VTEP reconcile via the NAD reconciler rather than a CNC event. Recomputing
	// on the VTEP path folds those late-joining networks into the exemption sets.
	if c.cncLister != nil {
		if err := c.reconcileCNCIsolationExemptions(); err != nil {
			return fmt.Errorf("failed to reconcile CNC isolation exemptions for VTEP %s: %w", vtep.Name, err)
		}
	}

	if err := c.reconcileOVSPorts(vtep.Name, GetEVPNBridgeName(vtep.Name), networks); err != nil {
		var linkNotFound netlink.LinkNotFoundError
		if errors.As(err, &linkNotFound) {
			klog.V(4).Infof("VTEP %s OVS port not yet available (%v), will retry", vtep.Name, err)
			c.vtepController.ReconcileRateLimited(vtep.Name)
			return nil
		}
		return fmt.Errorf("failed to reconcile VTEP %s OVS ports: %w", vtep.Name, err)
	}

	return nil
}

// ensureDevices programs NDM-managed devices for a VTEP: bridge, VXLANs, and SVIs.
// OVS ports are handled separately by reconcileOVSPorts.
func (c *Controller) ensureDevices(vtep *vtepv1.VTEP, vtepIPv4, vtepIPv6 net.IP) ([]evpnNetworkInfo, error) {
	bridgeName := GetEVPNBridgeName(vtep.Name)

	klog.V(4).Infof("Applying EVPN devices for VTEP %s: bridge=%s, IPv4=%v, IPv6=%v",
		vtep.Name, bridgeName, vtepIPv4, vtepIPv6)

	err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
		Link: &netlink.Bridge{
			LinkAttrs:       netlink.LinkAttrs{Name: bridgeName},
			VlanFiltering:   ptr.To(true),
			VlanDefaultPVID: ptr.To(uint16(0)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to apply VTEP %s bridge %s: %w", vtep.Name, bridgeName, err)
	}

	networks, err := c.collectEVPNNetworks(vtep.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to collect VTEP %s EVPN networks: %w", vtep.Name, err)
	}

	cncConfigs, err := c.collectCNCParentVRFConfigs(vtep.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to collect VTEP %s CNC parent VRF configs: %w", vtep.Name, err)
	}

	mappings := c.getVIDVNIMappings(networks, cncConfigs)
	if vtepIPv4 != nil {
		vxlan4Name := GetEVPNVXLANName(vtep.Name, utilnet.IPv4)
		if err := c.ensureVXLAN(vxlan4Name, bridgeName, vtepIPv4, mappings); err != nil {
			return nil, fmt.Errorf("failed to configure VTEP %s IPv4 VXLAN device: %w", vtep.Name, err)
		}
	} else {
		// Cover losing ipv4 vtep IP
		if err := c.ndm.DeleteLink(GetEVPNVXLANName(vtep.Name, utilnet.IPv4)); err != nil {
			return nil, fmt.Errorf("failed to delete VTEP %s IPv4 VXLAN device: %w", vtep.Name, err)
		}
	}

	if vtepIPv6 != nil {
		vxlan6Name := GetEVPNVXLANName(vtep.Name, utilnet.IPv6)
		if err := c.ensureVXLAN(vxlan6Name, bridgeName, vtepIPv6, mappings); err != nil {
			return nil, fmt.Errorf("failed to configure VTEP %s IPv6 VXLAN device: %w", vtep.Name, err)
		}
	} else {
		// Cover losing ipv6 vtep IP
		if err := c.ndm.DeleteLink(GetEVPNVXLANName(vtep.Name, utilnet.IPv6)); err != nil {
			return nil, fmt.Errorf("failed to delete VTEP %s IPv6 VXLAN device: %w", vtep.Name, err)
		}
	}

	// Reconcile CNC parent VRFs before SVIs: a MAC-VRF-only network's L2 SVI may be
	// enslaved to a CNC parent VRF (the parent acts as its IP-VRF), so the parent VRF
	// device must already exist when reconcileSVIs masters the SVI to it.
	if err := c.reconcileCNCParentVRFs(bridgeName, cncConfigs); err != nil {
		return nil, fmt.Errorf("failed to reconcile VTEP %s CNC parent VRFs: %w", vtep.Name, err)
	}

	macVRFParents, err := c.collectMACVRFParentVRFMap(vtep.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve VTEP %s MAC-VRF parent VRFs: %w", vtep.Name, err)
	}
	for i := range networks {
		if parent, ok := macVRFParents[networks[i].vrfName]; ok {
			networks[i].l2SVIMaster = parent
		}
	}

	if err := c.reconcileSVIs(bridgeName, networks); err != nil {
		return nil, fmt.Errorf("failed to reconcile VTEP %s SVIs: %w", vtep.Name, err)
	}

	// Program child-VRF override routes that steer inter-UDN egress toward the CNC
	// parent VRF (hub). This runs after the SVIs/parent VRFs so the devices the routes
	// reference are in flight; missing devices are handled by carry-forward inside.
	if err := c.reconcileCNCSiblingRoutes(vtep.Name); err != nil {
		return nil, fmt.Errorf("failed to reconcile VTEP %s CNC sibling routes: %w", vtep.Name, err)
	}

	return networks, nil
}

func (c *Controller) reconcileNAD(key string) error {
	netInfo := c.networkMgr.GetNetInfoForNADKey(key)
	c.nadVTEPInfoLock.Lock()
	defer c.nadVTEPInfoLock.Unlock()

	if netInfo == nil {
		vtepName, ok := c.nadVTEPInfo[key]
		if ok {
			klog.Infof("Network %s removed, reconciling VTEP %s", key, vtepName)
			delete(c.nadVTEPInfo, key)
			c.vtepController.ReconcileRateLimited(vtepName)
		}
		return nil
	}

	vtepName := netInfo.EVPNVTEPName()
	if vtepName == "" {
		return nil
	}

	if c.nadVTEPInfo[key] == vtepName {
		// NAD was already reconciled
		return nil
	}

	c.nadVTEPInfo[key] = vtepName
	c.vtepController.ReconcileRateLimited(vtepName)

	return nil
}

// evpnNetworkInfo holds EVPN config for a network.
type evpnNetworkInfo struct {
	macVRFVID, macVRFVNI int
	ipVRFVID, ipVRFVNI   int
	l3SVIName            string
	l2SVIName            string
	ovsPortName          string
	macVRFLSPName        string
	vrfName              string
	// l2SVIMaster is the VRF the L2 (MAC-VRF) SVI is enslaved to. It defaults to
	// vrfName (the network's own VRF). For a MAC-VRF-only (Layer2, no IP-VRF) network
	// connected by an EVPN CNC it is set to the CNC parent VRF, so the parent VRF acts
	// as the network's IP-VRF and FRR runs symmetric IRB for the L2-only CUDN over the
	// parent's L3 VNI. Empty is treated as vrfName by reconcileSVIs.
	l2SVIMaster string
}

// cncParentVRFInfo holds the desired Linux device state for a CNC parent VRF.
type cncParentVRFInfo struct {
	vrfName string
	sviName string
	vid     int
	vni     int
	tableID uint32
}

// cncNodeNeedsUpdate returns true when a CNC change is relevant to the node EVPN controller.
// Watches for spec changes (networkSelectors may change which networks/VTEPs are selected,
// which drives both the parent VRF import set and the isolation exemptions) and VID
// allocation (the readiness signal that triggers SVI creation). The connect-subnet
// annotation is not watched: it is only populated for Geneve-transport CNCs, so EVPN
// connected subnets come from each network's CUDN spec instead.
func cncNodeNeedsUpdate(oldObj, newObj *networkconnectv1.ClusterNetworkConnect) bool {
	if oldObj == nil || newObj == nil {
		return true
	}
	return oldObj.Generation != newObj.Generation ||
		oldObj.Annotations[util.OvnCNCEVPNParentVRFVIDAnnotation] != newObj.Annotations[util.OvnCNCEVPNParentVRFVIDAnnotation]
}

// collectCNCConnectedSubnets returns, per EVPN CNC, the pod subnets of every
// network the CNC connects, read from each connected network's CUDN spec
// (NetInfo.Subnets). EVPN CNCs do not use the connect-subnet annotation, which is
// only populated for Geneve-transport CNCs. Non-EVPN CNCs are skipped: their
// inter-UDN traffic stays inside OVN and is never re-injected through the host
// output hook, so it does not hit the udn drop chains.
func (c *Controller) collectCNCConnectedSubnets() (map[string][]*net.IPNet, error) {
	if c.cncLister == nil {
		return nil, nil
	}
	cncs, err := c.cncLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list CNCs: %w", err)
	}
	result := make(map[string][]*net.IPNet)
	for _, cnc := range cncs {
		if cnc.Spec.EVPNConfiguration == nil {
			continue
		}
		var subnets []*net.IPNet
		// vtepName is empty: exemptions are node-global, so gather subnets from every
		// EVPN network the CNC connects regardless of which VTEP it uses.
		_, err := c.forEachCNCEVPNNetwork(cnc, "", func(network util.NetInfo) bool {
			for _, entry := range network.Subnets() {
				if entry.CIDR != nil {
					subnets = append(subnets, entry.CIDR)
				}
			}
			return true
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve CNC %s connected networks: %w", cnc.Name, err)
		}
		if len(subnets) == 0 {
			continue
		}
		result[cnc.Name] = subnets
	}
	return result, nil
}

// reconcileCNCIsolationExemptions refreshes the nftables exemptions that let
// inter-UDN pod traffic between the networks a CNC connects bypass udn-bgp-drop.
func (c *Controller) reconcileCNCIsolationExemptions() error {
	cncSubnets, err := c.collectCNCConnectedSubnets()
	if err != nil {
		return err
	}
	return node.ReconcileCNCIsolationExemptions(cncSubnets)
}

// collectCNCParentVRFConfigs returns the desired CNC parent VRF state for a given VTEP.
// It resolves CNC networkSelectors locally using the NAD and namespace listers and the
// node's networkManager — no annotation on the CNC object is needed to determine VTEP membership.
// Returns nil when NetworkConnect is disabled (cncLister is not wired).
func (c *Controller) collectCNCParentVRFConfigs(vtepName string) ([]*cncParentVRFInfo, error) {
	if c.cncLister == nil {
		return nil, nil
	}
	cncs, err := c.cncLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list CNCs: %w", err)
	}

	var configs []*cncParentVRFInfo
	for _, cnc := range cncs {
		if cnc.Spec.EVPNConfiguration == nil {
			continue
		}
		vid, err := util.ParseNetworkConnectEVPNParentVRFVIDAnnotation(cnc)
		if err != nil {
			klog.Warningf("CNC %s: invalid VID annotation, skipping: %v", cnc.Name, err)
			continue
		}
		if vid == 0 {
			// VID not yet allocated by clustermanager; skip until ready.
			continue
		}

		// Resolve the CNC's networkSelectors to EVPN networks. This mirrors the logic in
		// the clustermanager's discoverSelectedNetworks, but uses the node's local listers.
		usesVTEP, err := c.cncUsesVTEP(cnc, vtepName)
		if err != nil {
			klog.Warningf("CNC %s: error resolving networks for VTEP %s, skipping: %v", cnc.Name, vtepName, err)
			continue
		}
		if !usesVTEP {
			continue
		}

		configs = append(configs, &cncParentVRFInfo{
			vrfName: util.GetCNCParentVRFName(cnc.Name),
			sviName: GetEVPNCNCParentVRFSVIName(cnc.Name),
			vid:     vid,
			vni:     int(cnc.Spec.EVPNConfiguration.IPVRF.VNI),
			tableID: uint32(cncParentVRFTableBase + vid),
		})
	}
	return configs, nil
}

// cncUsesVTEP returns true if any network selected by the CNC uses the given VTEP.
// It resolves the CNC's networkSelectors using the local NAD and namespace listers,
// then checks EVPNVTEPName() against vtepName via the networkManager.
func (c *Controller) cncUsesVTEP(cnc *networkconnectv1.ClusterNetworkConnect, vtepName string) (bool, error) {
	var uses bool
	_, err := c.forEachCNCEVPNNetwork(cnc, vtepName, func(util.NetInfo) bool {
		uses = true
		return false // stop at the first match
	})
	return uses, err
}

// forEachCNCEVPNNetwork resolves the CNC's ClusterUserDefinedNetworks selectors to
// EVPN networks using the local NAD lister and networkManager, and invokes fn for
// each EVPN network. When vtepName is non-empty, only networks using that VTEP are
// visited; when empty, every EVPN network the CNC connects is visited (used by the
// node-global isolation-exemption reconcile). fn returns false to stop iteration
// early. PrimaryUserDefinedNetworks selectors are ignored: namespace-scoped UDNs do
// not support EVPN transport.
//
// It also returns the number of selected CUDN-owned NADs whose NetInfo is not yet
// registered in the networkManager. Network registration lags NAD creation, so a
// NAD can be present while its network (and thus its pod subnets and VTEP binding)
// is still unknown. Callers that need the CNC's complete connected set (e.g. sibling
// route programming) use this count to defer and requeue rather than act on a partial
// view; callers that only probe for a match (e.g. cncUsesVTEP) may ignore it.
func (c *Controller) forEachCNCEVPNNetwork(cnc *networkconnectv1.ClusterNetworkConnect, vtepName string, fn func(util.NetInfo) bool) (int, error) {
	pending := 0
	for _, selector := range cnc.Spec.NetworkSelectors {
		if selector.NetworkSelectionType != apitypes.ClusterUserDefinedNetworks {
			continue
		}
		networkSelector, err := metav1.LabelSelectorAsSelector(&selector.ClusterUserDefinedNetworkSelector.NetworkSelector)
		if err != nil {
			return pending, fmt.Errorf("failed to parse CUDN selector: %w", err)
		}
		nads, err := c.nadLister.List(networkSelector)
		if err != nil {
			return pending, fmt.Errorf("failed to list NADs: %w", err)
		}
		for _, nad := range nads {
			owner := metav1.GetControllerOfNoCopy(nad)
			if owner == nil || owner.Kind != cudnGVK.Kind || owner.APIVersion != cudnGVK.GroupVersion().String() {
				continue
			}
			network := c.networkMgr.GetNetInfoForNADKey(nad.Namespace + "/" + nad.Name)
			if network == nil {
				// The NAD exists but its network is not yet registered in the
				// networkManager (registration lags NAD creation). Its subnets and
				// VTEP binding are unknown, so report it as pending.
				pending++
				continue
			}
			if network.EVPNVTEPName() == "" {
				// Not an EVPN network.
				continue
			}
			if vtepName != "" && network.EVPNVTEPName() != vtepName {
				continue
			}
			if !fn(network) {
				return pending, nil
			}
		}
	}
	return pending, nil
}

// collectMACVRFParentVRFMap returns, for each MAC-VRF-only (Layer2, no IP-VRF) EVPN
// network connected by an EVPN CNC and using vtepName, a mapping from the network's
// Linux VRF name to the connecting CNC's parent VRF name. Enslaving such a network's
// L2 (MAC-VRF) SVI to the parent VRF makes the parent act as the network's IP-VRF, so
// FRR performs symmetric IRB for the L2-only CUDN over the parent's L3 VNI. Networks
// with their own IP-VRF keep their own SVI master and are not included.
func (c *Controller) collectMACVRFParentVRFMap(vtepName string) (map[string]string, error) {
	if c.cncLister == nil {
		return nil, nil
	}
	cncs, err := c.cncLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list CNCs: %w", err)
	}
	result := make(map[string]string)
	for _, cnc := range cncs {
		if cnc.Spec.EVPNConfiguration == nil {
			continue
		}
		vid, err := util.ParseNetworkConnectEVPNParentVRFVIDAnnotation(cnc)
		if err != nil {
			klog.Warningf("CNC %s: invalid VID annotation, skipping MAC-VRF parent mapping: %v", cnc.Name, err)
			continue
		}
		if vid == 0 {
			// VID not yet allocated by clustermanager; parent VRF not ready.
			continue
		}
		parentVRF := util.GetCNCParentVRFName(cnc.Name)
		_, err = c.forEachCNCEVPNNetwork(cnc, vtepName, func(network util.NetInfo) bool {
			// MAC-VRF-only: has a MAC-VRF VID but no IP-VRF VID of its own.
			if network.EVPNMACVRFVID() != 0 && network.EVPNIPVRFVID() == 0 {
				result[util.GetNetworkVRFName(network)] = parentVRF
			}
			return true
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// reconcileCNCParentVRFs ensures the desired CNC parent VRF devices and SVIs exist
// on the given bridge, and removes stale ones. The VRF device is created first;
// the SVI is then mastered to it so FRR can perform EVPN import-vrf routing.
func (c *Controller) reconcileCNCParentVRFs(bridgeName string, configs []*cncParentVRFInfo) error {
	desiredSVIs := sets.New[string]()
	desiredVRFs := sets.New[string]()
	for _, cfg := range configs {
		desiredVRFs.Insert(cfg.vrfName)
		desiredSVIs.Insert(cfg.sviName)
		if err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
			Link: &netlink.Vrf{
				LinkAttrs: netlink.LinkAttrs{Name: cfg.vrfName},
				Table:     cfg.tableID,
			},
			// Cross-VRF inter-UDN transit is reinjected through this VRF master by
			// l3mdev. The reverse route for the source resolves out another VRF
			// master device (a table redirect), which loose RPF still rejects, so
			// reverse-path filtering must be fully disabled, not merely loosened.
			RPFilterDisable: true,
		}); err != nil {
			return fmt.Errorf("failed to ensure CNC parent VRF device %s: %w", cfg.vrfName, err)
		}
		if err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
			Link: &netlink.Vlan{
				LinkAttrs: netlink.LinkAttrs{Name: cfg.sviName},
				VlanId:    cfg.vid,
			},
			VLANParent: bridgeName,
			Master:     cfg.vrfName,
			// The parent VRF's L3VNI SVI is where VXLAN-decapped inter-UDN traffic
			// ingresses before being forwarded to a child VRF. The reverse route for
			// the source resolves out a child VRF master device, which loose RPF
			// rejects, so reverse-path filtering must be fully disabled here.
			RPFilterDisable: true,
		}); err != nil {
			return fmt.Errorf("failed to ensure CNC parent VRF SVI %s: %w", cfg.sviName, err)
		}
	}

	c.cncVRFsByBridgeLock.Lock()
	defer c.cncVRFsByBridgeLock.Unlock()

	for sviName := range c.cncVRFsByBridge[bridgeName] {
		if !desiredSVIs.Has(sviName) {
			if err := c.ndm.DeleteLink(sviName); err != nil {
				return fmt.Errorf("failed to delete stale CNC parent VRF SVI %s: %w", sviName, err)
			}
		}
	}
	c.cncVRFsByBridge[bridgeName] = desiredSVIs

	for vrfName := range c.cncVRFDevicesByBridge[bridgeName] {
		if !desiredVRFs.Has(vrfName) {
			if err := c.ndm.DeleteLink(vrfName); err != nil {
				return fmt.Errorf("failed to delete stale CNC parent VRF device %s: %w", vrfName, err)
			}
		}
	}
	c.cncVRFDevicesByBridge[bridgeName] = desiredVRFs

	return nil
}

// reconcileCNCSiblingRoutes programs, in each local EVPN child network's own VRF
// routing table, override routes that steer traffic destined to a sibling network's
// pod subnet into the CNC parent VRF (hub). The parent VRF then performs the
// inter-UDN lookup and forwards to the correct child, implementing hub-and-spoke
// routing without every child importing every other child's routes.
//
// Each connected network's pod subnets come from its CUDN spec (NetInfo.Subnets),
// not the connect-subnet annotation, which is only populated for Geneve-transport
// CNCs. For every local network N connected by the CNC we install, in N's VRF table,
// a route per sibling subnet with output device = the CNC parent VRF, so the kernel
// re-looks-up the packet in the parent VRF's table (Linux VRF leaking via a device
// route). These routes use a low, non-zero metric so a more-specific per-pod route
// imported by symmetric IRB still wins, while a leaked aggregate does not.
//
// The parent VRF device and the per-network management ports are created asynchronously
// (by NDM and the management-port controller respectively). When either is not yet
// present, the CNC's routes are deferred: any routes already programmed for that CNC are
// carried forward (kept in the store, not deleted) so they do not flap, and the reconcile
// returns nil for that CNC. The route manager's own periodic sync restores any route that
// is externally removed. Stale routes (a CNC's connected set shrank, or a network/VTEP
// was removed) are deleted.
func (c *Controller) reconcileCNCSiblingRoutes(vtepName string) error {
	if c.cncLister == nil {
		return nil
	}
	cncs, err := c.cncLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list CNCs: %w", err)
	}

	c.cncSiblingRoutesLock.Lock()
	defer c.cncSiblingRoutesLock.Unlock()
	stored := c.cncSiblingRoutesByVTEP[vtepName]

	desired := make(map[siblingRouteKey]siblingRouteEntry)
	// notReady tracks CNCs whose parent VRF device or a connected network's management
	// port is not yet present; their previously programmed routes are carried forward.
	notReady := sets.New[string]()

	for _, cnc := range cncs {
		if cnc.Spec.EVPNConfiguration == nil {
			continue
		}
		// The parent VRF device is created asynchronously by NDM; defer until it exists.
		parentVRFName := util.GetCNCParentVRFName(cnc.Name)
		parentLink, err := util.GetNetLinkOps().LinkByName(parentVRFName)
		if err != nil {
			var linkNotFound netlink.LinkNotFoundError
			if errors.As(err, &linkNotFound) {
				notReady.Insert(cnc.Name)
				continue
			}
			return fmt.Errorf("failed to resolve CNC %s parent VRF %s: %w", cnc.Name, parentVRFName, err)
		}

		// EVPN CNCs do not use the Geneve-only connect-subnet annotation. Each
		// connected network carries its real pod subnets in its CUDN spec, so read
		// them from NetInfo. Collect every network the CNC connects: its owner, ID
		// and subnets. The union of subnets per owner is the sibling set; each
		// network's routes exclude its own subnets.
		type cncNet struct {
			networkID int
			owner     string
		}
		var nets []cncNet
		ownerSubnets := make(map[string][]*net.IPNet)
		pending, err := c.forEachCNCEVPNNetwork(cnc, vtepName, func(network util.NetInfo) bool {
			networkID := network.GetNetworkID()
			owner := util.ComputeNetworkOwner(network.TopologyType(), networkID)
			nets = append(nets, cncNet{networkID: networkID, owner: owner})
			for _, entry := range network.Subnets() {
				if entry.CIDR != nil {
					ownerSubnets[owner] = append(ownerSubnets[owner], entry.CIDR)
				}
			}
			return true
		})
		if err != nil {
			return err
		}
		// A connected network's NAD exists but its NetInfo is not yet registered in the
		// networkManager, so its pod subnets are unknown. Programming now would install
		// an incomplete sibling set (missing routes to/from the unresolved network) and
		// nothing else re-triggers this reconcile once the network resolves, since the
		// NAD reconciler requeues the VTEP at most once per NAD. Defer the whole CNC:
		// carry forward any routes already programmed and requeue until every connected
		// network resolves.
		if pending > 0 {
			klog.V(4).Infof("VTEP %s: CNC %s has %d connected network(s) pending registration, deferring sibling routes", vtepName, cnc.Name, pending)
			notReady.Insert(cnc.Name)
			continue
		}

		// Program, in each local network's VRF table, a route to every sibling
		// network's subnet via the parent VRF. Accumulate this CNC's routes
		// separately so a partially-ready CNC (some management ports missing) is
		// deferred atomically rather than half-applied.
		cncRoutes := make(map[siblingRouteKey]siblingRouteEntry)
		cncNotReady := false
		for _, n := range nets {
			// A network whose ID is not yet allocated cannot have a stable management
			// port name; defer the whole CNC until every connected network resolves.
			if n.networkID == types.InvalidID {
				cncNotReady = true
				break
			}
			// The child network's VRF routing table is keyed off its management port's
			// ifindex; the management-port controller creates it asynchronously.
			mgmtPortName := util.GetNetworkScopedK8sMgmtHostIntfName(uint(n.networkID))
			mgmtLink, err := util.GetNetLinkOps().LinkByName(mgmtPortName)
			if err != nil {
				var linkNotFound netlink.LinkNotFoundError
				if errors.As(err, &linkNotFound) {
					cncNotReady = true
					break
				}
				return fmt.Errorf("failed to resolve network %d management port %s: %w", n.networkID, mgmtPortName, err)
			}
			tableID := util.CalculateRouteTableID(mgmtLink.Attrs().Index)

			for owner, subnets := range ownerSubnets {
				if owner == n.owner {
					continue
				}
				for _, subnet := range subnets {
					route := netlink.Route{
						Dst:       subnet,
						Table:     tableID,
						LinkIndex: parentLink.Attrs().Index,
						Priority:  cncSiblingRouteMetric,
						Protocol:  netlink.RouteProtocol(types.OVNKProtocol),
					}
					cncRoutes[siblingRouteKey{dst: subnet.String(), table: tableID}] = siblingRouteEntry{route: route, cnc: cnc.Name}
				}
			}
		}
		if cncNotReady {
			notReady.Insert(cnc.Name)
			continue
		}
		maps.Copy(desired, cncRoutes)
	}

	// Carry forward routes owned by not-ready CNCs so they are neither re-added
	// (their parent VRF/management port may be gone) nor deleted (avoid flapping).
	for k, v := range stored {
		if notReady.Has(v.cnc) {
			if _, ok := desired[k]; !ok {
				desired[k] = v
			}
		}
	}

	newStore := make(map[siblingRouteKey]siblingRouteEntry)
	var errs []error
	for k, v := range desired {
		if notReady.Has(v.cnc) {
			// Already programmed on a prior reconcile; keep without touching netlink.
			newStore[k] = v
			continue
		}
		if err := c.routeManager.Add(v.route); err != nil {
			errs = append(errs, fmt.Errorf("failed to add CNC %s sibling route %s: %w", v.cnc, v.route.Dst, err))
			continue
		}
		newStore[k] = v
	}
	for k, v := range stored {
		if _, ok := desired[k]; ok {
			continue
		}
		if err := c.routeManager.Del(v.route); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete stale CNC %s sibling route %s: %w", v.cnc, v.route.Dst, err))
			newStore[k] = v // keep to retry deletion
			continue
		}
	}

	if len(newStore) == 0 {
		delete(c.cncSiblingRoutesByVTEP, vtepName)
	} else {
		c.cncSiblingRoutesByVTEP[vtepName] = newStore
	}

	// The parent VRF device and per-network management ports are created
	// asynchronously (NDM and the management-port controller). When a CNC is
	// deferred because one of those is not yet present, nothing else will
	// re-trigger this reconcile once they appear, so requeue the VTEP with backoff
	// until the deferred CNCs become programmable.
	if notReady.Len() > 0 {
		klog.V(4).Infof("VTEP %s: %d CNC(s) not ready for sibling routes (parent VRF or management port pending), requeuing", vtepName, notReady.Len())
		c.vtepController.ReconcileRateLimited(vtepName)
	}

	return utilerrors.Join(errs...)
}

// collectEVPNNetworks gathers EVPN network info for all networks using this VTEP.
// This is rebuilt on every reconcile to pick up network additions/removals without
// maintaining a separate long-lived cache.
func (c *Controller) collectEVPNNetworks(vtepName string) ([]evpnNetworkInfo, error) {
	var networks []evpnNetworkInfo

	err := c.networkMgr.DoWithLock(func(netInfo util.NetInfo) error {
		if netInfo == nil || netInfo.EVPNVTEPName() != vtepName {
			return nil
		}
		switchName := netInfo.GetNetworkScopedSwitchName(types.OVNLayer2Switch)
		networks = append(networks, evpnNetworkInfo{
			macVRFVID:     netInfo.EVPNMACVRFVID(),
			macVRFVNI:     int(netInfo.EVPNMACVRFVNI()),
			ipVRFVID:      netInfo.EVPNIPVRFVID(),
			ipVRFVNI:      int(netInfo.EVPNIPVRFVNI()),
			l3SVIName:     GetEVPNL3SVIName(netInfo),
			l2SVIName:     GetEVPNL2SVIName(netInfo),
			ovsPortName:   GetEVPNOVSPortName(netInfo),
			macVRFLSPName: util.GetMACVRFPortName(switchName),
			vrfName:       util.GetNetworkVRFName(netInfo),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return networks, nil
}

// getVIDVNIMappings builds the VID-VNI mapping table from the collected networks.
// Returns a non-nil empty slice when no networks exist so NDM removes stale mappings.
func (c *Controller) getVIDVNIMappings(networks []evpnNetworkInfo, cncConfigs []*cncParentVRFInfo) []netlinkdevicemanager.VIDVNIMapping {
	mappings := make([]netlinkdevicemanager.VIDVNIMapping, 0)
	for _, n := range networks {
		if n.macVRFVID != 0 && n.macVRFVNI != 0 {
			mappings = append(mappings, netlinkdevicemanager.VIDVNIMapping{VID: uint16(n.macVRFVID), VNI: uint32(n.macVRFVNI)})
		}
		if n.ipVRFVID != 0 && n.ipVRFVNI != 0 {
			mappings = append(mappings, netlinkdevicemanager.VIDVNIMapping{VID: uint16(n.ipVRFVID), VNI: uint32(n.ipVRFVNI)})
		}
	}
	for _, cfg := range cncConfigs {
		if cfg.vid != 0 && cfg.vni != 0 {
			mappings = append(mappings, netlinkdevicemanager.VIDVNIMapping{VID: uint16(cfg.vid), VNI: uint32(cfg.vni)})
		}
	}
	return mappings
}

// reconcileSVIs ensures desired SVIs exist and removes stale ones.
// Creates L3 (IP-VRF) SVIs for routing and L2 (MAC-VRF) SVIs for ARP suppression
// and L2VNI-to-VRF association. Tracks created SVIs in svisByBridge to detect
// stale ones.
func (c *Controller) reconcileSVIs(bridgeName string, networks []evpnNetworkInfo) error {
	desiredSVIs := sets.New[string]()
	for _, net := range networks {
		if net.ipVRFVID != 0 {
			desiredSVIs.Insert(net.l3SVIName)
			if err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
				Link: &netlink.Vlan{
					LinkAttrs: netlink.LinkAttrs{Name: net.l3SVIName},
					VlanId:    net.ipVRFVID,
				},
				VLANParent: bridgeName,
				Master:     net.vrfName,
				// Inter-UDN traffic forwarded across VRFs ingresses on this SVI and
				// its reverse route resolves out another VRF master device, which
				// loose RPF rejects; reverse-path filtering must be fully disabled.
				RPFilterDisable: true,
			}); err != nil {
				return fmt.Errorf("failed to ensure L3 SVI %s: %w", net.l3SVIName, err)
			}
		}

		if net.macVRFVID != 0 {
			desiredSVIs.Insert(net.l2SVIName)
			// The L2 SVI is normally enslaved to the network's own VRF. For a
			// MAC-VRF-only network connected by an EVPN CNC, l2SVIMaster points at the
			// CNC parent VRF so the parent VRF becomes the network's IP-VRF.
			l2Master := net.l2SVIMaster
			if l2Master == "" {
				l2Master = net.vrfName
			}
			if err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
				Link: &netlink.Vlan{
					LinkAttrs: netlink.LinkAttrs{Name: net.l2SVIName},
					VlanId:    net.macVRFVID,
				},
				VLANParent: bridgeName,
				Master:     l2Master,
				// When enslaved to a CNC parent VRF this SVI is the L3 entry point for
				// symmetric IRB and carries asymmetric cross-VRF traffic whose reverse
				// route resolves out a VRF master device; reverse-path filtering must be
				// fully disabled, since loose RPF rejects that reverse path.
				RPFilterDisable: true,
			}); err != nil {
				return fmt.Errorf("failed to ensure L2 SVI %s: %w", net.l2SVIName, err)
			}
		}
	}

	c.svisByBridgeLock.Lock()
	defer c.svisByBridgeLock.Unlock()
	for sviName := range c.svisByBridge[bridgeName] {
		if !desiredSVIs.Has(sviName) {
			if err := c.ndm.DeleteLink(sviName); err != nil {
				return fmt.Errorf("failed to delete stale SVI %s: %w", sviName, err)
			}
		}
	}
	c.svisByBridge[bridgeName] = desiredSVIs

	return nil
}

// reconcileOVSPorts ensures OVS internal ports exist for MAC-VRF networks and removes stale ones.
func (c *Controller) reconcileOVSPorts(vtepName, bridgeName string, networks []evpnNetworkInfo) error {
	desiredPorts := sets.New[string]()
	for _, net := range networks {
		if net.macVRFVID == 0 {
			continue
		}
		desiredPorts.Insert(net.ovsPortName)
		if err := c.ensureOVSPort(vtepName, bridgeName, net.ovsPortName, net.macVRFLSPName, net.macVRFVID); err != nil {
			return fmt.Errorf("failed to ensure OVS port %s: %w", net.ovsPortName, err)
		}
	}

	ports, err := libovsdbops.FindOVSPortsWithPredicate(c.ovsClient, func(port *vswitchd.Port) bool {
		return port.ExternalIDs[types.EVPNVTEPExternalID] == vtepName
	})
	if err != nil {
		return fmt.Errorf("failed to list OVS ports for VTEP %s: %w", vtepName, err)
	}
	for _, port := range ports {
		if !desiredPorts.Has(port.Name) {
			klog.Infof("OVS port %s no longer needed for VTEP %s, removing", port.Name, vtepName)
			if err := libovsdbops.DeletePortWithInterfaces(c.ovsClient, ovsBridgeInt, port.Name); err != nil {
				return fmt.Errorf("failed to delete stale OVS port %s: %w", port.Name, err)
			}
		}
	}

	return nil
}

// ensureOVSPort creates an OVS internal port on br-int, attaches it to the EVPN bridge, and sets VLAN access.
// The iface-id on the Interface external_ids is set to macVRFLSPName so that ovn-controller can bind the
// corresponding MAC-VRF logical switch port to this OVS port.
func (c *Controller) ensureOVSPort(vtepName, bridgeName, portName, macVRFLSPName string, vid int) error {
	if err := libovsdbops.CreateOrUpdatePortWithInterface(c.ovsClient, ovsBridgeInt, portName,
		map[string]string{types.EVPNVTEPExternalID: vtepName},
		map[string]string{"iface-id": macVRFLSPName}); err != nil {
		return fmt.Errorf("failed to create OVS port %s: %w", portName, err)
	}

	ovsLink, err := util.GetNetLinkOps().LinkByName(portName)
	if err != nil {
		return fmt.Errorf("failed to get OVS port link %s: %w", portName, err)
	}
	evpnBridge, err := util.GetNetLinkOps().LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("failed to get EVPN bridge %s: %w", bridgeName, err)
	}
	if err := util.GetNetLinkOps().LinkSetMaster(ovsLink, evpnBridge); err != nil {
		return fmt.Errorf("failed to attach OVS port %s to bridge %s: %w", portName, bridgeName, err)
	}
	if err := util.GetNetLinkOps().LinkSetUp(ovsLink); err != nil {
		return fmt.Errorf("failed to bring up OVS port %s: %w", portName, err)
	}

	if err := util.GetNetLinkOps().BridgeVlanAdd(ovsLink, uint16(vid), true, true, false, true); err != nil {
		return fmt.Errorf("failed to set VLAN %d on OVS port %s: %w", vid, portName, err)
	}

	return nil
}

func (c *Controller) deleteVTEPDevices(vtepName string) error {
	bridgeName := GetEVPNBridgeName(vtepName)
	var errs []error

	ports, err := libovsdbops.FindOVSPortsWithPredicate(c.ovsClient, func(port *vswitchd.Port) bool {
		return port.ExternalIDs[types.EVPNVTEPExternalID] == vtepName
	})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list VTEP %s OVS ports: %w", vtepName, err))
	}
	for _, port := range ports {
		if err := libovsdbops.DeletePortWithInterfaces(c.ovsClient, ovsBridgeInt, port.Name); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VTEP %s OVS port %s: %w", vtepName, port.Name, err))
		}
	}

	// Delete all SVIs parented to this bridge
	c.svisByBridgeLock.Lock()
	for sviName := range c.svisByBridge[bridgeName] {
		if err := c.ndm.DeleteLink(sviName); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VTEP %s SVI %s: %w", vtepName, sviName, err))
		} else {
			c.svisByBridge[bridgeName].Delete(sviName)
		}
	}
	if c.svisByBridge[bridgeName].Len() == 0 {
		delete(c.svisByBridge, bridgeName)
	}
	c.svisByBridgeLock.Unlock()

	// Clean up CNC parent VRF SVIs and VRF devices associated with this bridge.
	c.cncVRFsByBridgeLock.Lock()
	for sviName := range c.cncVRFsByBridge[bridgeName] {
		if err := c.ndm.DeleteLink(sviName); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VTEP %s CNC parent VRF SVI %s: %w", vtepName, sviName, err))
		}
	}
	delete(c.cncVRFsByBridge, bridgeName)
	for vrfName := range c.cncVRFDevicesByBridge[bridgeName] {
		if err := c.ndm.DeleteLink(vrfName); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VTEP %s CNC parent VRF device %s: %w", vtepName, vrfName, err))
		}
	}
	delete(c.cncVRFDevicesByBridge, bridgeName)
	c.cncVRFsByBridgeLock.Unlock()

	// Remove the child-VRF sibling routes programmed for this VTEP's networks.
	c.cncSiblingRoutesLock.Lock()
	for _, entry := range c.cncSiblingRoutesByVTEP[vtepName] {
		if err := c.routeManager.Del(entry.route); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VTEP %s CNC sibling route %s: %w", vtepName, entry.route.Dst, err))
		}
	}
	delete(c.cncSiblingRoutesByVTEP, vtepName)
	c.cncSiblingRoutesLock.Unlock()

	if err := c.ndm.DeleteLink(GetEVPNVXLANName(vtepName, utilnet.IPv4)); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete VTEP %s IPv4 VXLAN device: %w", vtepName, err))
	}
	if err := c.ndm.DeleteLink(GetEVPNVXLANName(vtepName, utilnet.IPv6)); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete VTEP %s IPv6 VXLAN device: %w", vtepName, err))
	}
	if err := c.ndm.DeleteLink(bridgeName); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete VTEP %s bridge %s: %w", vtepName, bridgeName, err))
	}

	c.nadVTEPInfoLock.Lock()
	for key, name := range c.nadVTEPInfo {
		if name == vtepName {
			delete(c.nadVTEPInfo, key)
		}
	}
	c.nadVTEPInfoLock.Unlock()

	return utilerrors.Join(errs...)
}

// cleanupStaleOVSPorts removes OVS ports tagged with the evpn-vtep external-id
// whose VTEP no longer exists. This handles VTEPs deleted while the controller was down,
// which the per-VTEP reconcile loop cannot catch.
func (c *Controller) cleanupStaleOVSPorts(activeVTEPs sets.Set[string]) error {
	ports, err := libovsdbops.FindOVSPortsWithPredicate(c.ovsClient, func(port *vswitchd.Port) bool {
		vtep, ok := port.ExternalIDs[types.EVPNVTEPExternalID]
		return ok && !activeVTEPs.Has(vtep)
	})
	if err != nil {
		return fmt.Errorf("failed to list stale OVS ports: %w", err)
	}
	var errs []error
	for _, port := range ports {
		klog.Infof("Cleaning up stale OVS port %s (VTEP %s no longer exists)", port.Name, port.ExternalIDs[types.EVPNVTEPExternalID])
		if err := libovsdbops.DeletePortWithInterfaces(c.ovsClient, ovsBridgeInt, port.Name); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete stale OVS port %s: %w", port.Name, err))
		}
	}
	return utilerrors.Join(errs...)
}

// ensureVXLAN programs a VXLAN device on the EVPN bridge.
func (c *Controller) ensureVXLAN(vxlanName string, bridgeName string, srcIP net.IP, mappings []netlinkdevicemanager.VIDVNIMapping) error {
	err := c.ndm.EnsureLink(netlinkdevicemanager.DeviceConfig{
		Link: &netlink.Vxlan{
			LinkAttrs: netlink.LinkAttrs{Name: vxlanName, MTU: config.Default.MTU},
			Port:      config.DefaultVXLANPort,
			SrcAddr:   srcIP,
			// FlowBased (ip link: external): destination VTEP is resolved per-flow from the FDB, populated by FRR via BGP EV
			FlowBased: true,
			// Only accept traffic for configured VNIs
			VniFilter: true,
		},
		Master: bridgeName,
		BridgePortSettings: &netlinkdevicemanager.BridgePortSettings{
			VLANTunnel: true,
			// Answer ARP/ND locally from the bridge neigh table instead of flooding
			NeighSuppress: true,
			// Disable data-plane MAC learning, rely on BGP EVPN Type-2 routes
			Learning: false,
			// Isolated VXLAN ports can only forward to non-isolated ports.
			// In dual-stack setups with separate IPv4 and IPv6 VXLAN devices on the
			// same bridge, BUM traffic arriving on one VXLAN would be flooded out
			// the other, creating an amplification loop between VTEP peers.
			Isolated: true,
		},
		VIDVNIMappings: mappings,
	})
	if err != nil {
		return fmt.Errorf("failed to apply VXLAN %s: %w", vxlanName, err)
	}
	return nil
}

// discoverUnmanagedVTEPIPs resolves the IPs to use for an unmanaged VTEP.
// For each IP family present in the VTEP's CIDRs, it checks the node's VTEP annotation
// for a previously selected IP. If that IP is still present in the address manager and
// within the VTEP's CIDRs, it is reused for stability. Otherwise, a new IP is discovered
// from the address manager and the annotation is updated.
func (c *Controller) discoverUnmanagedVTEPIPs(vtep *vtepv1.VTEP) (net.IP, net.IP, error) {
	// fetch the annotated VTEP IPs
	vtepsAnnotation, err := c.getVTEPsAnnotation()
	if err != nil {
		return nil, nil, err
	}
	v4AnnotatedIP := net.ParseIP(matchFirstIPStringFamily(false, vtepsAnnotation[vtep.Name].IPs))
	v6AnnotatedIP := net.ParseIP(matchFirstIPStringFamily(true, vtepsAnnotation[vtep.Name].IPs))

	// get valid node IP addresses
	nodeIPs, _ := c.addressManager.ListAddresses()
	v4NodeIPs, v6NodeIPs := util.SplitIPsByIPFamily(nodeIPs)

	// get the VTEP CIDRs
	vtepIPNets, err := util.ParseIPNets(vtep.Spec.CIDRs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse VTEP %q CIDRs %v: %w", vtep.Name, vtep.Spec.CIDRs, err)
	}
	v4VTEPCIDRS, v6VTEPCIDRS := util.SplitIPNetsByIPFamily(vtepIPNets)

	// reuse the annotated IP if still valid otherwise select a new one,
	// resolving each IP family independently
	v4SelectedIP, err := c.resolveVTEPIP(netlink.FAMILY_V4, v4VTEPCIDRS, v4NodeIPs, v4AnnotatedIP)
	if err != nil {
		return nil, nil, err
	}

	v6SelectedIP, err := c.resolveVTEPIP(netlink.FAMILY_V6, v6VTEPCIDRS, v6NodeIPs, v6AnnotatedIP)
	if err != nil {
		return nil, nil, err
	}

	changed := !v4AnnotatedIP.Equal(v4SelectedIP) || !v6AnnotatedIP.Equal(v6SelectedIP)
	if changed {
		if err := c.setVTEPAnnotation(vtep.Name, v4SelectedIP, v6SelectedIP); err != nil {
			return nil, nil, fmt.Errorf("failed to annotate VTEP %s IPs: %w", vtep.Name, err)
		}
	}
	return v4SelectedIP, v6SelectedIP, nil
}

// resolveVTEPIP returns the IP to use for a given family. If the VTEP IP
// is still present among the node IPs and within the VTEP's CIDRs, it is
// returned. Otherwise, a new IP is selected from matching node IPs.
func (c *Controller) resolveVTEPIP(family int, vtepCIDRs []*net.IPNet, nodeIPs []net.IP, vtepIP net.IP) (net.IP, error) {
	if len(vtepCIDRs) == 0 {
		return nil, nil
	}

	equalsVTEPIP := func(ip net.IP) bool { return ip.Equal(vtepIP) }
	if vtepIP != nil && slices.ContainsFunc(nodeIPs, equalsVTEPIP) && util.IsIPContainedInAnyCIDR(vtepIP, vtepCIDRs...) {
		return vtepIP, nil
	}

	// Select a new IP from node addresses matching the CIDRs.
	var matches []net.IP
	for _, nodeIP := range nodeIPs {
		if util.IsIPContainedInAnyCIDR(nodeIP, vtepCIDRs...) {
			matches = append(matches, nodeIP)
		}
	}
	return c.pickVTEPIP(matches, family)
}

// getVTEPsAnnotation returns the controller-local copy of the VTEP annotation.
// On first call it bootstraps from the informer cache; subsequent calls return
// the local copy which is kept in sync by setVTEPAnnotation. This avoids stale
// reads when multiple VTEPs are reconciled before the informer cache updates.
func (c *Controller) getVTEPsAnnotation() (map[string]util.VTEPNodeAnnotation, error) {
	if c.vtepsAnnotation != nil {
		return c.vtepsAnnotation, nil
	}
	node, err := c.watchFactory.GetNode(c.nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", c.nodeName, err)
	}
	vtepsAnnotation, err := util.ParseNodeVTEPs(node)
	if err != nil {
		if util.IsAnnotationNotSetError(err) {
			c.vtepsAnnotation = map[string]util.VTEPNodeAnnotation{}
			return c.vtepsAnnotation, nil
		}
		return nil, fmt.Errorf("failed to parse VTEP annotation: %w", err)
	}
	c.vtepsAnnotation = vtepsAnnotation
	return c.vtepsAnnotation, nil
}

// setVTEPAnnotation updates or removes a VTEP entry in the node's VTEP
// annotation. If both IPs are nil, the entry is removed; otherwise it is
// set to the provided IPs.
func (c *Controller) setVTEPAnnotation(vtepName string, ipv4, ipv6 net.IP) (err error) {
	vtepsAnnotation, err := c.getVTEPsAnnotation()
	if err != nil {
		return err
	}

	// restore previous value on error to ensure retry will reattempt
	previous := vtepsAnnotation[vtepName]
	defer func() {
		if err == nil {
			return
		}
		vtepsAnnotation[vtepName] = previous
		if len(previous.IPs) == 0 {
			delete(vtepsAnnotation, vtepName)
		}
	}()

	switch {
	case ipv4 == nil && ipv6 == nil:
		delete(vtepsAnnotation, vtepName)
	default:
		entry := util.VTEPNodeAnnotation{}
		if ipv4 != nil {
			entry.IPs = append(entry.IPs, ipv4.String())
		}
		if ipv6 != nil {
			entry.IPs = append(entry.IPs, ipv6.String())
		}
		vtepsAnnotation[vtepName] = entry
	}

	// inhibit API request if noop
	if slices.Equal(slices.Sorted(slices.Values(previous.IPs)), slices.Sorted(slices.Values(vtepsAnnotation[vtepName].IPs))) {
		return nil
	}

	annotations, err := util.MarshalNodeVTEPs(vtepsAnnotation)
	if err != nil {
		return err
	}

	return c.kube.SetAnnotationsOnNodeWithFieldManager(c.nodeName, annotations, vtepAnnotationFieldManager)
}

// pickVTEPIP selects a single VTEP IP from the candidates for the given address family.
// If there's exactly one match, it's used directly. Multiple matches trigger a netlink
// lookup to filter out keepalived VIPs and secondary addresses (which can float between
// nodes). If ambiguity remains, the lexicographically lowest IP is chosen for determinism.
func (c *Controller) pickVTEPIP(matches []net.IP, family int) (net.IP, error) {
	if len(matches) <= 1 {
		if len(matches) == 1 {
			return matches[0], nil
		}
		return nil, nil
	}

	klog.Infof("Multiple VTEP IP candidates %v (family %d), filtering VIPs via netlink", matches, family)
	addrs, err := util.GetNetLinkOps().AddrList(nil, family)
	if err != nil {
		return nil, fmt.Errorf("failed to list addresses (family %d): %w", family, err)
	}
	skipIPs := map[string]bool{}
	for _, addr := range addrs {
		if util.IsAddressAddedByKeepAlived(addr) || (addr.Flags&unix.IFA_F_SECONDARY) != 0 {
			skipIPs[addr.IP.String()] = true
		}
	}
	filtered := matches[:0]
	for _, ip := range matches {
		if !skipIPs[ip.String()] {
			filtered = append(filtered, ip)
		}
	}
	slices.SortFunc(filtered, func(a, b net.IP) int {
		addrA, _ := netip.AddrFromSlice(a)
		addrB, _ := netip.AddrFromSlice(b)
		return addrA.Compare(addrB)
	})
	if len(filtered) > 0 {
		return filtered[0], nil
	}
	return nil, nil

}

func matchFirstIPStringFamily(isIPv6 bool, ips []string) string {
	ip, err := util.MatchIPStringFamily(isIPv6, ips)
	if err != nil {
		return ""
	}
	return ip
}
