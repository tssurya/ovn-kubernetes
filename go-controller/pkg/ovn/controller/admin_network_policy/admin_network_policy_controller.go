package admin_network_policy

import (
	"fmt"
	"time"

	libovsdbclient "github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	anpapi "sigs.k8s.io/network-policy-api/apis/v1alpha1"
	anpinformer "sigs.k8s.io/network-policy-api/client/informers/externalversions/apis/v1alpha1"
	anplister "sigs.k8s.io/network-policy-api/client/listers/apis/v1alpha1"
)

const (
	// maxRetries is the number of times a object will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the
	// sequence of delays between successive queuings of an object.
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15

	controllerName = "admin-network-policy-controller"
)

type Controller struct {
	client kubernetes.Interface

	// libovsdb northbound client interface
	nbClient      libovsdbclient.Client
	eventRecorder record.EventRecorder

	anpQueue        workqueue.RateLimitingInterface
	banpQueue       workqueue.RateLimitingInterface
	anpLister       anplister.AdminNetworkPolicyLister
	banpLister      anplister.BaselineAdminNetworkPolicyLister
	anpCacheSynced  cache.InformerSynced
	banpCacheSynced cache.InformerSynced
}

// NewController returns a new *Controller.
func NewController(
	client kubernetes.Interface,
	nbClient libovsdbclient.Client,
	anpInformer anpinformer.AdminNetworkPolicyInformer,
	banpInformer anpinformer.BaselineAdminNetworkPolicyInformer,
	recorder record.EventRecorder) *Controller {

	c := &Controller{
		client:   client,
		nbClient: nbClient,
	}

	klog.Info("Setting up event handlers for Admin Network Policy")
	// setup anp informers, listers, queue
	anpInformer.Informer().AddEventHandler(factory.WithUpdateHandlingForObjReplace(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onANPAdd,
		UpdateFunc: c.onANPUpdate,
		DeleteFunc: c.onANPDelete,
	}))
	c.anpLister = anpInformer.Lister()
	c.anpCacheSynced = anpInformer.Informer().HasSynced
	c.anpQueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.NewItemFastSlowRateLimiter(1*time.Second, 5*time.Second, 5),
		"adminNetworkPolicy",
	)
	klog.Info("Setting up event handlers for Baseline Admin Network Policy")
	// setup banp informers, listers, queue
	banpInformer.Informer().AddEventHandler(factory.WithUpdateHandlingForObjReplace(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onBANPAdd,
		UpdateFunc: c.onBANPUpdate,
		DeleteFunc: c.onBANPDelete,
	}))
	c.banpLister = banpInformer.Lister()
	c.banpCacheSynced = banpInformer.Informer().HasSynced
	c.banpQueue = workqueue.NewNamedRateLimitingQueue(
		workqueue.NewItemFastSlowRateLimiter(1*time.Second, 5*time.Second, 5),
		"baselineAdminNetworkPolicy",
	)

	c.eventRecorder = recorder

	return c
}

// Run will not return until stopCh is closed. workers determines how many
// endpoints will be handled in parallel.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.anpQueue.ShutDown()
	defer c.banpQueue.ShutDown()

	klog.Infof("Starting controller %s", controllerName)
	defer klog.Infof("Shutting down controller %s", controllerName)

	// Wait for the caches to be synced
	klog.Info("Waiting for informer caches to sync")
	if !cache.WaitForNamedCacheSync(controllerName, stopCh, c.anpCacheSynced, c.banpCacheSynced) {
		return fmt.Errorf("error syncing caches for admin network policy and baseline admin network policy")
	}

	/*if runRepair {
		// Run the repair controller only once
		// it keeps in sync Kubernetes and OVN
		// and handles removal of stale data on upgrades
		c.repair.runBeforeSync()
	}*/

	// Start the workers after the repair loop to avoid races
	klog.Info("Starting Admin Network Policy workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runANPWorker, time.Second, stopCh)
	}

	klog.Info("Starting Baseline Admin Network Policy workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runBANPWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and
// marks them done. You may run as many of these in parallel as you wish; the
// workqueue guarantees that they will not end up processing the same service
// at the same time.
func (c *Controller) runANPWorker() {
	for c.processNextANPWorkItem() {
	}
}

func (c *Controller) runBANPWorker() {
	for c.processNextBANPWorkItem() {
	}
}

func (c *Controller) processNextANPWorkItem() bool {
	anpKey, quit := c.anpQueue.Get()
	if quit {
		return false
	}
	defer c.anpQueue.Done(anpKey)

	err := c.syncAdminNetworkPolicy(anpKey.(string))
	if err == nil {
		c.anpQueue.Forget(anpKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", anpKey, err))

	if c.anpQueue.NumRequeues(anpKey) < maxRetries {
		c.anpQueue.AddRateLimited(anpKey)
		return true
	}

	c.anpQueue.Forget(anpKey)
	return true
}

func (c *Controller) processNextBANPWorkItem() bool {
	banpKey, quit := c.banpQueue.Get()
	if quit {
		return false
	}
	defer c.banpQueue.Done(banpKey)

	err := c.syncBaselineAdminNetworkPolicy(banpKey.(string))
	if err == nil {
		c.banpQueue.Forget(banpKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", banpKey, err))

	if c.banpQueue.NumRequeues(banpKey) < maxRetries {
		c.banpQueue.AddRateLimited(banpKey)
		return true
	}

	c.banpQueue.Forget(banpKey)
	return true
}

func (c *Controller) syncAdminNetworkPolicy(key string) error {
	return nil
}

func (c *Controller) syncBaselineAdminNetworkPolicy(key string) error {
	return nil
}

// handlers

// onANPAdd queues the ANP for processing.
func (c *Controller) onANPAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	klog.V(4).Infof("Adding Admin Network Policy %s", key)
	c.anpQueue.Add(key)
}

// onANPUpdate updates the ANP Selector in the cache and queues the ANP for processing.
func (c *Controller) onANPUpdate(oldObj, newObj interface{}) {
	oldANP := oldObj.(*anpapi.AdminNetworkPolicy)
	newANP := newObj.(*anpapi.AdminNetworkPolicy)

	// don't process resync or objects that are marked for deletion
	if oldANP.ResourceVersion == newANP.ResourceVersion ||
		!newANP.GetDeletionTimestamp().IsZero() {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err == nil {
		c.anpQueue.Add(key)
	}
}

// onANPDelete queues the ANP for processing.
func (c *Controller) onANPDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	klog.V(4).Infof("Deleting Admin Network Policy %s", key)
	c.anpQueue.Add(key)
}

// onBANPAdd queues the BANP for processing.
func (c *Controller) onBANPAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	klog.V(4).Infof("Adding Baseline Admin Network Policy %s", key)
	c.banpQueue.Add(key)
}

// onBANPUpdate updates the BANP Selector in the cache and queues the ANP for processing.
func (c *Controller) onBANPUpdate(oldObj, newObj interface{}) {
	oldBANP := oldObj.(*anpapi.BaselineAdminNetworkPolicy)
	newBANP := newObj.(*anpapi.BaselineAdminNetworkPolicy)

	// don't process resync or objects that are marked for deletion
	if oldBANP.ResourceVersion == newBANP.ResourceVersion ||
		!newBANP.GetDeletionTimestamp().IsZero() {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err == nil {
		c.banpQueue.Add(key)
	}
}

// onBANPDelete queues the BANP for processing.
func (c *Controller) onBANPDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	klog.V(4).Infof("Deleting Baseline Admin Network Policy %s", key)
	c.banpQueue.Add(key)
}
