// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package evpn

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	networkconnectv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1"
	networkconnectlisters "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1/apis/listers/clusternetworkconnect/v1"
	"github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/util"
)

// connectSubnetAnnotation is the CNC annotation the clustermanager writes with the
// per-network connected subnets. Kept in sync with pkg/util/network_connect.go.
const connectSubnetAnnotation = "k8s.ovn.org/network-connect-subnet"

// subnetAnnotationFor builds a connect-subnet annotation value for the given
// per-network CIDRs, mirroring util.UpdateNetworkConnectSubnetAnnotation's format.
func subnetAnnotationFor(t *testing.T, perNetwork map[string]util.NetworkConnectSubnetAnnotation) string {
	t.Helper()
	bytes, err := json.Marshal(perNetwork)
	require.NoError(t, err)
	return string(bytes)
}

func newCNCListerFrom(cncs ...*networkconnectv1.ClusterNetworkConnect) networkconnectlisters.ClusterNetworkConnectLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, cnc := range cncs {
		_ = indexer.Add(cnc)
	}
	return networkconnectlisters.NewClusterNetworkConnectLister(indexer)
}

func TestCollectCNCConnectedSubnets(t *testing.T) {
	evpnCNC := &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{
			Name: "evpn-connect",
			Annotations: map[string]string{
				connectSubnetAnnotation: subnetAnnotationFor(t, map[string]util.NetworkConnectSubnetAnnotation{
					"alpha": {IPv4: "10.10.0.0/16"},
					"beta":  {IPv4: "10.20.0.0/16", IPv6: "2001:db8:20::/64"},
				}),
			},
		},
		Spec: networkconnectv1.ClusterNetworkConnectSpec{
			EVPNConfiguration: &networkconnectv1.EVPNCNCConfig{},
		},
	}
	// Non-EVPN CNC with subnets: routed inside OVN, must be excluded.
	nonEVPNCNC := &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ovn-connect",
			Annotations: map[string]string{
				connectSubnetAnnotation: subnetAnnotationFor(t, map[string]util.NetworkConnectSubnetAnnotation{
					"gamma": {IPv4: "10.30.0.0/16"},
				}),
			},
		},
	}
	// EVPN CNC without a subnet annotation yet: nothing to exempt.
	evpnNoSubnets := &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{Name: "evpn-pending"},
		Spec: networkconnectv1.ClusterNetworkConnectSpec{
			EVPNConfiguration: &networkconnectv1.EVPNCNCConfig{},
		},
	}

	c := &Controller{cncLister: newCNCListerFrom(evpnCNC, nonEVPNCNC, evpnNoSubnets)}

	result, err := c.collectCNCConnectedSubnets()
	require.NoError(t, err)

	// Only the EVPN CNC with subnets is present, and its per-network subnets are flattened.
	require.Len(t, result, 1)
	subnets := result["evpn-connect"]
	require.Len(t, subnets, 3)
	cidrs := make([]string, 0, len(subnets))
	for _, s := range subnets {
		cidrs = append(cidrs, s.String())
	}
	assert.ElementsMatch(t, []string{"10.10.0.0/16", "10.20.0.0/16", "2001:db8:20::/64"}, cidrs)
}

func TestCollectCNCConnectedSubnets_NoLister(t *testing.T) {
	c := &Controller{}
	result, err := c.collectCNCConnectedSubnets()
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestCNCNodeNeedsUpdate(t *testing.T) {
	base := &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cnc",
			Generation: 1,
			Annotations: map[string]string{
				util.OvnCNCEVPNParentVRFVIDAnnotation: "5",
				connectSubnetAnnotation:               `{"alpha":{"ipv4":"10.10.0.0/16"}}`,
			},
		},
	}
	clone := func(mutate func(*networkconnectv1.ClusterNetworkConnect)) *networkconnectv1.ClusterNetworkConnect {
		cp := base.DeepCopy()
		mutate(cp)
		return cp
	}

	tests := []struct {
		name string
		new  *networkconnectv1.ClusterNetworkConnect
		want bool
	}{
		{
			name: "no change",
			new:  base.DeepCopy(),
			want: false,
		},
		{
			name: "generation bump",
			new:  clone(func(c *networkconnectv1.ClusterNetworkConnect) { c.Generation = 2 }),
			want: true,
		},
		{
			name: "VID annotation change",
			new:  clone(func(c *networkconnectv1.ClusterNetworkConnect) { c.Annotations[util.OvnCNCEVPNParentVRFVIDAnnotation] = "6" }),
			want: true,
		},
		{
			name: "connect-subnet annotation change",
			new:  clone(func(c *networkconnectv1.ClusterNetworkConnect) { c.Annotations[connectSubnetAnnotation] = `{"alpha":{"ipv4":"10.11.0.0/16"}}` }),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cncNodeNeedsUpdate(base, tt.new))
		})
	}
}
