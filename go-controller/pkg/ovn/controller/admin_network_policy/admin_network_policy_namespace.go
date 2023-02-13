package admin_network_policy

import (
	"fmt"
	"sync"
	"time"

	libovsdbclient "github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdbops"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/nbdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type mapOp int

const (
	mapInsert mapOp = iota
	mapDelete
)

type mapAndOp struct {
	m   *sync.Map
	key string
	val any
	op  mapOp
}

func (c *Controller) syncNamespaceAdminNetworkPolicy(key string) error {
	// Iterate all ANPs and check if this namespace start/stops matching
	// any and add/remove the setup accordingly. Namespaces can match multiple
	// ANPs objects, so continue iterating all ANP objects before finishing.
	startTime := time.Now()
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Processing sync for Namespace %s in Admin Network Policy controller", name)

	defer func() {
		klog.V(4).Infof("Finished syncing Namespace %s Admin Network Policy controller: took %v", name, time.Since(startTime))
	}()
	namespace, err := c.anpNamespaceLister.Get(name)
	if err != nil {
		return err
	}
	if namespace == nil {
		// TODO: Deal with namespace deletion
		return nil
	}
	podNamespaceLister := c.anpPodLister.Pods(namespace.Name)
	pods, err := podNamespaceLister.List(labels.Everything())
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	existingANPs, err := c.anpLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, anp := range existingANPs {
		anpObj, loaded := c.anpCache.Load(anp.Spec.Priority)
		if !loaded {
			continue
		}
		podMapOps := []mapAndOp{}
		anpCache := anpObj.(*adminNetworkPolicy)
		ops, err := c.constructNamespaceUpdateOps(anp.Name, namespace, anpCache, pods, podMapOps)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			continue
		}
		klog.Infof("SURYA %v", podMapOps)
		_, err = libovsdbops.TransactAndCheck(c.nbClient, ops)
		if err != nil {
			return err
		}

		anpCache.Lock()
		for _, mapOp := range podMapOps {
			switch mapOp.op {
			case mapInsert:
				mapOp.m.Store(mapOp.key, mapOp.val)
			case mapDelete:
				mapOp.m.Delete(mapOp.key)
			}
		}
		anpCache.Unlock()
	}
	return nil
}

func (c *Controller) constructNamespaceUpdateOps(anpName string, namespace *v1.Namespace, anpCache *adminNetworkPolicy, pods []*v1.Pod, podMapOps []mapAndOp) ([]ovsdb.Operation, error) {
	anpCache.RLock() // allow multiple pods to sync
	defer anpCache.RUnlock()
	if anpName != anpCache.name {
		return nil, nil // perhaps another object with same priority exists
	}
	if anpCache.stale { // was deleted or not created properly
		return nil, nil
	}
	var portsToAdd, portsToDelete []string
	ops := []ovsdb.Operation{}
	var err error
	namespaceLabels := labels.Set(namespace.Labels)
	for _, pod := range pods {
		logicalPortName := util.GetLogicalPortName(pod.Namespace, pod.Name)
		lsp := &nbdb.LogicalSwitchPort{Name: logicalPortName}
		lsp, err = libovsdbops.GetLogicalSwitchPort(c.nbClient, lsp)
		if err != nil {
			if errors.Is(err, libovsdbclient.ErrNotFound) {
				continue // reprocess it when LSP is created - whenever the next update for the pod comes
			}
			anpCache.RUnlock()
			return nil, fmt.Errorf("error retrieving logical switch port with Name %s "+
				" from libovsdb cache: %w", logicalPortName, err)
		}
		podIPs, err := util.GetPodIPsOfNetwork(pod, &util.DefaultNetInfo{})
		if errors.Is(err, util.ErrNoPodIPFound) {
			continue // reprocess it when it is updated with an IP
		}
		if err != nil {
			// TODO: Check this logic is correct
			return nil, err
		}
		_, loaded := anpCache.subject.pods.Load(pod.Name)
		if anpCache.subject.namespaceSelector.Matches(namespaceLabels) && !loaded {
			portsToAdd = append(portsToAdd, lsp.UUID)
			podMapOps = append(podMapOps, mapAndOp{anpCache.subject.pods, pod.Name, lsp.UUID, mapInsert})
		} else if !anpCache.subject.namespaceSelector.Matches(namespaceLabels) && loaded {
			portsToDelete = append(portsToDelete, lsp.UUID)
			podMapOps = append(podMapOps, mapAndOp{anpCache.subject.pods, pod.Name, lsp.UUID, mapDelete})
		}
		pgName, _ := getAdminNetworkPolicyPGName(anpCache.name, anpCache.priority)
		if len(portsToAdd) > 0 {
			ops, err = libovsdbops.AddPortsToPortGroupOps(c.nbClient, ops, pgName, portsToAdd...)
			if err != nil {
				return nil, err
			}
		}
		if len(portsToDelete) > 0 {
			ops, err = libovsdbops.DeletePortsFromPortGroupOps(c.nbClient, ops, pgName, portsToDelete...)
			if err != nil {
				return nil, err
			}
		}
		for _, rule := range anpCache.ingressRules {
			for _, peer := range rule.peers {
				_, loaded := peer.pods.Load(pod.Name)
				if peer.namespaceSelector.Matches(namespaceLabels) && !loaded {
					addrOps, err := rule.addrSet.AddIPsReturnOps(podIPs)
					if err != nil {
						return nil, err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapInsert})
				} else if !peer.namespaceSelector.Matches(namespaceLabels) && loaded {
					addrOps, err := rule.addrSet.DeleteIPsReturnOps(podIPs)
					if err != nil {
						return nil, err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapDelete})
				}
			}
		}
		for _, rule := range anpCache.egressRules {
			for _, peer := range rule.peers {
				_, loaded := peer.pods.Load(pod.Name)
				if peer.namespaceSelector.Matches(namespaceLabels) && !loaded {
					addrOps, err := rule.addrSet.AddIPsReturnOps(podIPs)
					if err != nil {
						return nil, err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapInsert})
				} else if !peer.namespaceSelector.Matches(namespaceLabels) && loaded {
					addrOps, err := rule.addrSet.DeleteIPsReturnOps(podIPs)
					if err != nil {
						return nil, err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapDelete})
				}
			}
		}

	}
	return ops, nil
}
