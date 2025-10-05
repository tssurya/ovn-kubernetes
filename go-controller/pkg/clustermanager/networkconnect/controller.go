package networkconnect

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclientset "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	nadlisters "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/listers/k8s.cni.cncf.io/v1"
	controllerutil "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/controller"
	networkconnectv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1"
	networkconnectclientset "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1/apis/clientset/versioned"
	networkconnectlisters "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1/apis/listers/clusternetworkconnect/v1"
	userdefinednetworkv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/userdefinednetwork/v1"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/networkmanager"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
)

var (
	cudnController = userdefinednetworkv1.SchemeGroupVersion.WithKind("ClusterUserDefinedNetwork")
	udnController  = userdefinednetworkv1.SchemeGroupVersion.WithKind("UserDefinedNetwork")
)

type Controller struct {
	// wf is the watch factory for accessing informers
	wf *factory.WatchFactory
	// listers
	cncLister       networkconnectlisters.ClusterNetworkConnectLister
	nodeLister      corelisters.NodeLister
	namespaceLister corelisters.NamespaceLister
	nadLister       nadlisters.NetworkAttachmentDefinitionLister
	//clientset
	cncClient networkconnectclientset.Interface
	nadClient nadclientset.Interface
	// Controller for managing cluster-network-connect events
	cncController controllerutil.Controller
	// Controller for managing node events
	nodeController controllerutil.Controller
	// Controller for managing NetworkAttachmentDefinition events
	nadController  controllerutil.Controller
	networkManager networkmanager.Interface
}

func NewController(
	wf *factory.WatchFactory,
	ovnClient *util.OVNClusterManagerClientset,
	networkManager networkmanager.Interface,
) *Controller {
	cncLister := wf.ClusterNetworkConnectInformer().Lister()
	nadLister := wf.NADInformer().Lister()
	nodeLister := wf.NodeCoreInformer().Lister()
	c := &Controller{
		wf:              wf,
		cncClient:       ovnClient.NetworkConnectClient,
		nadClient:       ovnClient.NetworkAttchDefClient,
		cncLister:       cncLister,
		nadLister:       nadLister,
		nodeLister:      nodeLister,
		namespaceLister: wf.NamespaceInformer().Lister(),
		networkManager:  networkManager,
	}

	cncCfg := &controllerutil.ControllerConfig[networkconnectv1.ClusterNetworkConnect]{
		RateLimiter:    workqueue.DefaultTypedControllerRateLimiter[string](),
		Informer:       wf.ClusterNetworkConnectInformer().Informer(),
		Lister:         cncLister.List,
		Reconcile:      c.reconcileClusterNetworkConnect,
		ObjNeedsUpdate: cncNeedsUpdate,
		Threadiness:    1,
	}
	c.cncController = controllerutil.NewController(
		"clustermanager-network-connect-controller",
		cncCfg,
	)

	nadCfg := &controllerutil.ControllerConfig[nadv1.NetworkAttachmentDefinition]{
		RateLimiter:    workqueue.DefaultTypedControllerRateLimiter[string](),
		Informer:       wf.UserDefinedNetworkInformer().Informer(),
		Lister:         nadLister.List,
		Reconcile:      c.reconcileNAD,
		ObjNeedsUpdate: nadNeedsUpdate,
		Threadiness:    1,
	}
	c.nadController = controllerutil.NewController(
		"clustermanager-network-connect-network-attachment-definition-controller",
		nadCfg,
	)

	nodeCfg := &controllerutil.ControllerConfig[corev1.Node]{
		RateLimiter:    workqueue.DefaultTypedControllerRateLimiter[string](),
		Informer:       wf.NodeCoreInformer().Informer(),
		Lister:         nodeLister.List,
		Reconcile:      c.reconcileNode,
		ObjNeedsUpdate: nodeNeedsUpdate,
		Threadiness:    1,
	}
	c.nodeController = controllerutil.NewController(
		"clustermanager-network-connect-node-controller",
		nodeCfg,
	)

	return c
}

func (c *Controller) Start() error {
	defer klog.Infof("Cluster manager network connect controllers started")
	return controllerutil.Start(
		c.cncController,
		c.nadController,
		c.nodeController,
	)
}

func (c *Controller) Stop() {
	controllerutil.Stop(
		c.cncController,
		c.nadController,
		c.nodeController,
	)
	klog.Infof("Cluster manager network connect controllers stopped")
}

func cncNeedsUpdate(oldObj, newObj *networkconnectv1.ClusterNetworkConnect) bool {
	if oldObj == nil || newObj == nil {
		return true
	}
	// Only trigger updates when the Spec changes
	return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
}

func nadNeedsUpdate(oldObj, newObj *nadv1.NetworkAttachmentDefinition) bool {
	nadSupported := func(nad *nadv1.NetworkAttachmentDefinition) bool {
		if nad == nil {
			return false
		}
		// we don't support direct NADs anymore. CNC is only supported for CUDNs and UDNs
		controller := metav1.GetControllerOfNoCopy(nad)
		isCUDN := controller != nil && controller.Kind == cudnController.Kind && controller.APIVersion == cudnController.GroupVersion().String()
		isUDN := controller != nil && controller.Kind == udnController.Kind && controller.APIVersion == udnController.GroupVersion().String()
		if !isCUDN && !isUDN {
			return false
		}
		network, err := util.ParseNADInfo(newObj)
		if err != nil {
			return true
		}
		if network.IsPrimaryNetwork() {
			// only layer3 and layer2 topology are supported
			// but since primary network is always layer3 or layer2,
			// we can ignored the need to check the topology
			return true
		}
		return false // we don't support secondary networks, so we can ignore it
	}
	// ignore if we don't support this NAD
	if !nadSupported(oldObj) && !nadSupported(newObj) {
		return false
	}
	oldNADLabels := labels.Set{}
	newNADLabels := labels.Set{}
	if oldObj != nil {
		oldNADLabels = labels.Set(oldObj.Labels)
	}
	if newObj != nil {
		newNADLabels = labels.Set(newObj.Labels)
	}
	// Only trigger NAD updates when the object is created or deleted (UDNs->NADs)
	// Only trigger NAD updates when the labels change (CUDNs->NADs)
	return oldObj == nil || newObj == nil || !labels.Equals(oldNADLabels, newNADLabels)
}

func nodeNeedsUpdate(oldObj, newObj *corev1.Node) bool {
	// Only trigger node updates when the object is created or deleted
	// we can ignore node updates
	return oldObj == nil || newObj == nil
}

func (c *Controller) reconcileClusterNetworkConnect(key string) error {
	// STEP1: Validate the CNC
	// STEP2: Discover the selected UDNs and CUDNs
	// STEP3: Generate /31 subnets for each node-network pair for layer3 networks and one /31 subnet for each layer2 networks
	// STEP4: Generate a tunnelID for the connect router corresponding to this CNC

	return nil
}

func (c *Controller) reconcileNAD(key string) error {
	return nil
}

func (c *Controller) reconcileNode(key string) error {
	return nil
}
