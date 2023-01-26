package clustermanager

import (
	"fmt"
	"reflect"

	egressipv1 "github.com/ovn-org/ovn-kubernetes/go-controller/pkg/crd/egressip/v1"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/retry"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	kapi "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type cmEventHandler struct {
	retry.EventHandler

	objType         reflect.Type
	cm              *ClusterManager
	extraParameters interface{}
	syncFunc        func([]interface{}) error
}

func (cm *ClusterManager) newRetryFramework(objectType reflect.Type) *retry.RetryFramework {
	resourceHandler := &retry.ResourceHandler{
		HasUpdateFunc:          hasResourceAnUpdateFunc(objectType),
		NeedsUpdateDuringRetry: false,
		ObjType:                objectType,
		EventHandler: &cmEventHandler{
			objType:         objectType,
			cm:              cm,
			extraParameters: nil,
			syncFunc:        nil,
		},
	}
	r := retry.NewRetryFramework(cm.stopChan, cm.wg, cm.watchFactory, resourceHandler)
	return r
}

// hasResourceAnUpdateFunc returns true if the given resource type has a dedicated update function.
// It returns false if, upon an update event on this resource type, we instead need to first delete the old
// object and then add the new one.
func hasResourceAnUpdateFunc(objType reflect.Type) bool {
	switch objType {
	case factory.NodeType,
		  factory.EgressNodeType,
		  factory.EgressIPType:
		return true
	}
	return false
}

func (h *cmEventHandler) AreResourcesEqual(obj1, obj2 interface{}) (bool, error) {
	// switch based on type
	switch h.objType {
	case factory.NodeType:
		node1, ok := obj1.(*kapi.Node)
		if !ok {
			return false, fmt.Errorf("could not cast obj1 of type %T to *kapi.Node", obj1)
		}
		node2, ok := obj2.(*kapi.Node)
		if !ok {
			return false, fmt.Errorf("could not cast obj2 of type %T to *kapi.Node", obj2)
		}

		// when shouldUpdateNode is false, the hostsubnet is not assigned by ovn-kubernetes
		shouldUpdate, err := shouldUpdateNode(node2, node1)
		if err != nil {
			klog.Errorf(err.Error())
		}
		return !shouldUpdate, nil
	}

	return false, nil
}

// Given an object and its type, GetInternalCacheEntry returns the internal cache entry for this object.
func (h *cmEventHandler) GetInternalCacheEntry(obj interface{}) interface{} {
	return nil
}

// Given an object key and its type, getResourceFromInformerCache returns the latest state of the object
// from the informers cache.
func (h *cmEventHandler) GetResourceFromInformerCache(key string) (interface{}, error) {
	var obj interface{}
	var name string
	var err error

	_, name, err = cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to split key %s: %v", key, err)
	}

	switch h.objType {
	case factory.NodeType,
		factory.EgressNodeType:
		obj, err = h.cm.watchFactory.GetNode(name)
	case factory.EgressIPType:
		obj, err = h.cm.watchFactory.GetEgressIP(name)

	default:
		err = fmt.Errorf("object type %s not supported, cannot retrieve it from informers cache",
			h.objType)
	}
	return obj, err
}

// Given an object and its type, RecordAddEvent records the add event on this object.
func (h *cmEventHandler) RecordAddEvent(obj interface{}) {
}

// Given an object and its type, RecordUpdateEvent records the update event on this object.
func (h *cmEventHandler) RecordUpdateEvent(obj interface{}) {
}

// Given an object and its type, RecordDeleteEvent records the delete event on this object. Only used for pods now.
func (h *cmEventHandler) RecordDeleteEvent(obj interface{}) {
}

func (h *cmEventHandler) RecordSuccessEvent(obj interface{}) {
}

// Given an object and its type, RecordErrorEvent records an error event on this object.
// Only used for pods now.
func (h *cmEventHandler) RecordErrorEvent(obj interface{}, reason string, err error) {
}

// Given an object and its type, isResourceScheduled returns true if the object has been scheduled.
func (h *cmEventHandler) IsResourceScheduled(obj interface{}) bool {
	return true
}

// Given an object and its type, IsObjectInTerminalState returns true if the object is a in terminal state.
func (h *cmEventHandler) IsObjectInTerminalState(obj interface{}) bool {
	return false
}

// Given a *RetryFramework instance, an object to add and a boolean specifying if the function was executed from
// iterateRetryResources, AddResource adds the specified object to the cluster according to its type and
// returns the error, if any, yielded during object creation.
func (h *cmEventHandler) AddResource(obj interface{}, fromRetryLoop bool) error {
	var err error

	switch h.objType {
	case factory.NodeType:
		node, ok := obj.(*kapi.Node)
		if !ok {
			return fmt.Errorf("could not cast %T object to *kapi.Node", obj)
		}
		if err = h.cm.addUpdateNodeEvent(node); err != nil {
			klog.Infof("Node add failed for %s, will try again later: %v",
				node.Name, err)
			return err
		}
	case factory.EgressNodeType:
		node := obj.(*kapi.Node)
		if err := h.cm.setupNodeForEgress(node); err != nil {
			return err
		}
		nodeEgressLabel := util.GetNodeEgressLabel()
		nodeLabels := node.GetLabels()
		_, hasEgressLabel := nodeLabels[nodeEgressLabel]
		if hasEgressLabel {
			h.cm.setNodeEgressAssignable(node.Name, true)
		}
		isReady := h.cm.isEgressNodeReady(node)
		if isReady {
			h.cm.setNodeEgressReady(node.Name, true)
		}
		isReachable := h.cm.isEgressNodeReachable(node)
		if hasEgressLabel && isReachable && isReady {
			h.cm.setNodeEgressReachable(node.Name, true)
			if err := h.cm.addEgressNode(node.Name); err != nil {
				return err
			}
		}
	case factory.EgressIPType:
		eIP := obj.(*egressipv1.EgressIP)
		return h.cm.reconcileEgressIP(nil, eIP)
	default:
		return fmt.Errorf("no add function for object type %s", h.objType)
	}
	return nil
}

// Given a *RetryFramework instance, an old and a new object, UpdateResource updates the specified object in the cluster
// to its version in newObj according to its type and returns the error, if any, yielded during the object update.
// The inRetryCache boolean argument is to indicate if the given resource is in the retryCache or not.
func (h *cmEventHandler) UpdateResource(oldObj, newObj interface{}, inRetryCache bool) error {
	var err error

	switch h.objType {
	case factory.NodeType:
		node, ok := newObj.(*kapi.Node)
		if !ok {
			return fmt.Errorf("could not cast %T object to *kapi.Node", newObj)
		}
		if err = h.cm.addUpdateNodeEvent(node); err != nil {
			klog.Infof("Node update failed for %s, will try again later: %v",
				node.Name, err)
			return err
		}
	case factory.EgressIPType:
		oldEIP := oldObj.(*egressipv1.EgressIP)
		newEIP := newObj.(*egressipv1.EgressIP)
		return h.cm.reconcileEgressIP(oldEIP, newEIP)
	case factory.EgressNodeType:
		oldNode := oldObj.(*kapi.Node)
		newNode := newObj.(*kapi.Node)
		// Initialize the allocator on every update,
		// ovnkube-node/cloud-network-config-controller will make sure to
		// annotate the node with the egressIPConfig, but that might have
		// happened after we processed the ADD for that object, hence keep
		// retrying for all UPDATEs.
		if err := h.cm.initEgressIPAllocator(newNode); err != nil {
			klog.Warningf("Egress node initialization error: %v", err)
		}
		nodeEgressLabel := util.GetNodeEgressLabel()
		oldLabels := oldNode.GetLabels()
		newLabels := newNode.GetLabels()
		_, oldHadEgressLabel := oldLabels[nodeEgressLabel]
		_, newHasEgressLabel := newLabels[nodeEgressLabel]
		// If the node is not labeled for egress assignment, just return
		// directly, we don't really need to set the ready / reachable
		// status on this node if the user doesn't care about using it.
		if !oldHadEgressLabel && !newHasEgressLabel {
			return nil
		}
		h.cm.setNodeEgressAssignable(newNode.Name, newHasEgressLabel)
		if oldHadEgressLabel && !newHasEgressLabel {
			klog.Infof("Node: %s has been un-labeled, deleting it from egress assignment", newNode.Name)
			return h.cm.deleteEgressNode(oldNode.Name)
		}
		isOldReady := h.cm.isEgressNodeReady(oldNode)
		isNewReady := h.cm.isEgressNodeReady(newNode)
		isNewReachable := h.cm.isEgressNodeReachable(newNode)
		h.cm.setNodeEgressReady(newNode.Name, isNewReady)
		if !oldHadEgressLabel && newHasEgressLabel {
			klog.Infof("Node: %s has been labeled, adding it for egress assignment", newNode.Name)
			if isNewReady && isNewReachable {
				h.cm.setNodeEgressReachable(newNode.Name, isNewReachable)
				if err := h.cm.addEgressNode(newNode.Name); err != nil {
					return err
				}
			} else {
				klog.Warningf("Node: %s has been labeled, but node is not ready"+
					" and reachable, cannot use it for egress assignment", newNode.Name)
			}
			return nil
		}
		if isOldReady == isNewReady {
			return nil
		}
		if !isNewReady {
			klog.Warningf("Node: %s is not ready, deleting it from egress assignment", newNode.Name)
			if err := h.cm.deleteEgressNode(newNode.Name); err != nil {
				return err
			}
		} else if isNewReady && isNewReachable {
			klog.Infof("Node: %s is ready and reachable, adding it for egress assignment", newNode.Name)
			h.cm.setNodeEgressReachable(newNode.Name, isNewReachable)
			if err := h.cm.addEgressNode(newNode.Name); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("no update function for object type %s", h.objType)
	}
	return nil
}

// Given a *RetryFramework instance, an object and optionally a cachedObj, DeleteResource deletes the object from the cluster
// according to the delete logic of its resource type. cachedObj is the internal cache entry for this object,
// used for now for pods and network policies.
func (h *cmEventHandler) DeleteResource(obj, cachedObj interface{}) error {
	switch h.objType {
	case factory.NodeType:
		node, ok := obj.(*kapi.Node)
		if !ok {
			return fmt.Errorf("could not cast obj of type %T to *knet.Node", obj)
		}
		return h.cm.deleteNode(node)
	case factory.EgressIPType:
		eIP := obj.(*egressipv1.EgressIP)
		return h.cm.reconcileEgressIP(eIP, nil)
	case factory.EgressNodeType:
		node := obj.(*kapi.Node)
		if err := h.cm.deleteNodeForEgress(node); err != nil {
			return err
		}
		nodeEgressLabel := util.GetNodeEgressLabel()
		nodeLabels := node.GetLabels()
		_, hasEgressLabel := nodeLabels[nodeEgressLabel]
		if hasEgressLabel {
			if err := h.cm.deleteEgressNode(node.Name); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// deleteNodeForEgress remove the default allow logical router policies for the
// node and removes the node from the allocator cache.
func (cm *ClusterManager) deleteNodeForEgress(node *v1.Node) error {
	cm.eIPC.allocator.Lock()
	if eNode, exists := cm.eIPC.allocator.cache[node.Name]; exists {
		eNode.healthClient.Disconnect()
	}
	delete(cm.eIPC.allocator.cache, node.Name)
	cm.eIPC.allocator.Unlock()	
	return nil
}

func (h *cmEventHandler) SyncFunc(objs []interface{}) error {
	var syncFunc func([]interface{}) error

	if h.syncFunc != nil {
		// syncFunc was provided explicitly
		syncFunc = h.syncFunc
	} else {
		switch h.objType {
		case factory.NodeType:
			syncFunc = h.cm.syncNodes
		case factory.EgressNodeType:
			syncFunc = h.cm.initClusterEgressPolicies
		case factory.EgressIPType:
			syncFunc = nil
		default:
			return fmt.Errorf("no sync function for object type %s", h.objType)
		}
	}
	if syncFunc == nil {
		return nil
	}
	return syncFunc(objs)
}
