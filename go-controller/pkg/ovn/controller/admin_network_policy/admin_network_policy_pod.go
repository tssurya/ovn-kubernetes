package admin_network_policy

import (
	"fmt"
	"time"

	libovsdbclient "github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/libovsdbops"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/nbdb"
	"github.com/ovn-org/ovn-kubernetes/go-controller/pkg/util"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) syncPodAdminNetworkPolicy(key string) error {
	startTime := time.Now()
	// Iterate all ANPs and check if this namespace start/stops matching
	// any and add/remove the setup accordingly. Namespaces can match multiple
	// ANPs objects, so continue iterating all ANP objects before finishing.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	klog.V(4).Infof("Processing sync for Pod %s/%s in Admin Network Policy controller", namespace, name)

	defer func() {
		klog.V(4).Infof("Finished syncing Pod %s/%s Admin Network Policy controller: took %v", namespace, name, time.Since(startTime))
	}()
	ns, err := c.anpNamespaceLister.Get(namespace)
	if err != nil {
		return err
	}
	namespaceLabels := labels.Set(ns.Labels)
	podNamespaceLister := c.anpPodLister.Pods(namespace)
	pod, err := podNamespaceLister.Get(name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	existingANPs, err := c.anpLister.List(labels.Everything())
	if err != nil {
		return err
	}
	ops := []ovsdb.Operation{}
	if pod == nil || util.PodCompleted(pod) {
		// TODO: FIX THIS
		return nil
	}
	for _, anp := range existingANPs {
		podMapOps := []mapAndOp{}
		anpObj, loaded := c.anpCache.Load(anp.Spec.Priority)
		if !loaded {
			continue
		}
		anpCache := anpObj.(*adminNetworkPolicy)
		anpCache.RLock() // allow multiple pods to sync
		defer anpCache.RUnlock()
		if anpCache.stale { // was deleted or not created properly
			continue
		}
		if anp.Name != anpCache.name {
			continue // perhaps another object with same priority exists for which setup is not done by OVNK, skip it
		}
		var portsToAdd, portsToDelete []string
		logicalPortName := util.GetLogicalPortName(pod.Namespace, pod.Name)
		lsp := &nbdb.LogicalSwitchPort{Name: logicalPortName}
		lsp, err = libovsdbops.GetLogicalSwitchPort(c.nbClient, lsp)
		if err != nil {
			if errors.Is(err, libovsdbclient.ErrNotFound) {
				continue // reprocess it when LSP is created - whenever the next update for the pod comes
			}
			return fmt.Errorf("error retrieving logical switch port with Name %s "+
				" from libovsdb cache: %w", logicalPortName, err)
		}
		podIPs, err := util.GetPodIPsOfNetwork(pod, &util.DefaultNetInfo{})
		if errors.Is(err, util.ErrNoPodIPFound) {
			continue // reprocess it when it is updated with an IP
		}
		if err != nil {
			return err
		}
		podLabels := labels.Set(pod.Labels)
		if anpCache.subject.namespaceSelector.Matches(namespaceLabels) { // proceed only if namespace selectors are matching for the pod and ANP Subject
			_, loaded = anpCache.subject.pods.Load(pod.Name)
			if anpCache.subject.podSelector.Matches(podLabels) && !loaded {
				portsToAdd = append(portsToAdd, lsp.UUID)
				podMapOps = append(podMapOps, mapAndOp{anpCache.subject.pods, pod.Name, lsp.UUID, mapInsert})
			} else if !anpCache.subject.podSelector.Matches(podLabels) && loaded {
				portsToDelete = append(portsToDelete, lsp.UUID)
				podMapOps = append(podMapOps, mapAndOp{anpCache.subject.pods, pod.Name, lsp.UUID, mapDelete})
			}
		}
		for _, rule := range anpCache.ingressRules {
			for _, peer := range rule.peers {
				// proceed only if namespace selectors are matching for the pod and ANP Peer
				if !peer.namespaceSelector.Matches(namespaceLabels) {
					continue
				}
				_, loaded = peer.pods.Load(pod.Name)
				if peer.podSelector.Matches(podLabels) && !loaded {
					addrOps, err := rule.addrSet.AddIPsReturnOps(podIPs)
					if err != nil {
						return err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapInsert})
				} else if !peer.podSelector.Matches(podLabels) && loaded {
					addrOps, err := rule.addrSet.DeleteIPsReturnOps(podIPs)
					if err != nil {
						return err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapDelete})
				}
			}
		}
		for _, rule := range anpCache.egressRules {
			for _, peer := range rule.peers {
				// proceed only if namespace selectors are matching for the pod and ANP Peer
				if !peer.namespaceSelector.Matches(namespaceLabels) {
					continue
				}
				_, loaded = peer.pods.Load(pod.Name)
				if peer.podSelector.Matches(podLabels) && !loaded {
					addrOps, err := rule.addrSet.AddIPsReturnOps(podIPs)
					if err != nil {
						return err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapInsert})
				} else if !peer.podSelector.Matches(podLabels) && loaded {
					addrOps, err := rule.addrSet.DeleteIPsReturnOps(podIPs)
					if err != nil {
						return err
					}
					ops = append(ops, addrOps...)
					podMapOps = append(podMapOps, mapAndOp{peer.pods, pod.Name, podIPs, mapDelete})
				}
			}
		}
		pgName, _ := getAdminNetworkPolicyPGName(anpCache.name, anpCache.priority)
		if len(portsToAdd) > 0 {
			ops, err = libovsdbops.AddPortsToPortGroupOps(c.nbClient, ops, pgName, portsToAdd...)
			if err != nil {
				return err
			}
		}
		if len(portsToDelete) > 0 {
			ops, err = libovsdbops.DeletePortsFromPortGroupOps(c.nbClient, ops, pgName, portsToDelete...)
			if err != nil {
				return err
			}
		}
		_, err = libovsdbops.TransactAndCheck(c.nbClient, ops)
		if err != nil {
			return err
		}

		// TODO: FIX THIS, lock is ready only here
		for _, mapOp := range podMapOps {
			switch mapOp.op {
			case mapInsert:
				mapOp.m.Store(mapOp.key, mapOp.val)
			case mapDelete:
				mapOp.m.Delete(mapOp.key)
			}
		}
	}
	return nil
}
