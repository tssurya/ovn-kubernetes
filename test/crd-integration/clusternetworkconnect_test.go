// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package crdintegration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkconnectv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/clusternetworkconnect/v1"
	crdtypes "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/types"
)

// cncWithConnectSubnets builds a minimal Geneve-transport CNC using connectSubnets.
func cncWithConnectSubnets(name string) *networkconnectv1.ClusterNetworkConnect {
	return &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: networkconnectv1.ClusterNetworkConnectSpec{
			NetworkSelectors: crdtypes.NetworkSelectors{
				{
					NetworkSelectionType:              crdtypes.ClusterUserDefinedNetworks,
					ClusterUserDefinedNetworkSelector: &crdtypes.ClusterUserDefinedNetworkSelector{},
				},
			},
			ConnectSubnets: []networkconnectv1.ConnectSubnet{
				{CIDR: "10.128.0.0/16", NetworkPrefix: 24},
			},
			Connectivity: []networkconnectv1.ConnectivityType{networkconnectv1.PodNetwork},
		},
	}
}

// cncWithEVPN builds a minimal EVPN-transport CNC using evpnConfiguration.
func cncWithEVPN(name string, vni int32) *networkconnectv1.ClusterNetworkConnect {
	return &networkconnectv1.ClusterNetworkConnect{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: networkconnectv1.ClusterNetworkConnectSpec{
			NetworkSelectors: crdtypes.NetworkSelectors{
				{
					NetworkSelectionType:              crdtypes.ClusterUserDefinedNetworks,
					ClusterUserDefinedNetworkSelector: &crdtypes.ClusterUserDefinedNetworkSelector{},
				},
			},
			EVPNConfiguration: &networkconnectv1.EVPNCNCConfig{
				IPVRF: crdtypes.VRFConfig{VNI: vni},
			},
			Connectivity: []networkconnectv1.ConnectivityType{networkconnectv1.PodNetwork},
		},
	}
}

var _ = Describe("ClusterNetworkConnect CRD", func() {
	Context("connectSubnets / evpnConfiguration mutual exclusion", func() {
		It("accepts connectSubnets without evpnConfiguration (Geneve transport)", func() {
			cnc := cncWithConnectSubnets("cnc-geneve-ok")
			Expect(k8sClient.Create(ctx, cnc)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cnc) })
		})

		It("accepts evpnConfiguration without connectSubnets (EVPN transport)", func() {
			cnc := cncWithEVPN("cnc-evpn-ok", 5000)
			Expect(k8sClient.Create(ctx, cnc)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cnc) })
		})

		It("rejects a spec with neither connectSubnets nor evpnConfiguration", func() {
			cnc := &networkconnectv1.ClusterNetworkConnect{
				ObjectMeta: metav1.ObjectMeta{Name: "cnc-neither"},
				Spec: networkconnectv1.ClusterNetworkConnectSpec{
					NetworkSelectors: crdtypes.NetworkSelectors{
						{
							NetworkSelectionType:              crdtypes.ClusterUserDefinedNetworks,
							ClusterUserDefinedNetworkSelector: &crdtypes.ClusterUserDefinedNetworkSelector{},
						},
					},
					Connectivity: []networkconnectv1.ConnectivityType{networkconnectv1.PodNetwork},
				},
			}
			Expect(k8sClient.Create(ctx, cnc)).NotTo(Succeed())
		})

		It("rejects a spec with both connectSubnets and evpnConfiguration set", func() {
			cnc := &networkconnectv1.ClusterNetworkConnect{
				ObjectMeta: metav1.ObjectMeta{Name: "cnc-both"},
				Spec: networkconnectv1.ClusterNetworkConnectSpec{
					NetworkSelectors: crdtypes.NetworkSelectors{
						{
							NetworkSelectionType:              crdtypes.ClusterUserDefinedNetworks,
							ClusterUserDefinedNetworkSelector: &crdtypes.ClusterUserDefinedNetworkSelector{},
						},
					},
					ConnectSubnets: []networkconnectv1.ConnectSubnet{
						{CIDR: "10.128.0.0/16", NetworkPrefix: 24},
					},
					EVPNConfiguration: &networkconnectv1.EVPNCNCConfig{
						IPVRF: crdtypes.VRFConfig{VNI: 5000},
					},
					Connectivity: []networkconnectv1.ConnectivityType{networkconnectv1.PodNetwork},
				},
			}
			Expect(k8sClient.Create(ctx, cnc)).NotTo(Succeed())
		})
	})

	Context("evpnConfiguration.ipVRF.vni validation", func() {
		It("accepts the minimum valid VNI (1)", func() {
			cnc := cncWithEVPN("cnc-vni-min", 1)
			Expect(k8sClient.Create(ctx, cnc)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cnc) })
		})

		It("accepts the maximum valid VNI (16777215)", func() {
			cnc := cncWithEVPN("cnc-vni-max", 16777215)
			Expect(k8sClient.Create(ctx, cnc)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, cnc) })
		})

		It("rejects VNI 0 (below minimum)", func() {
			cnc := cncWithEVPN("cnc-vni-zero", 0)
			Expect(k8sClient.Create(ctx, cnc)).NotTo(Succeed())
		})

		It("rejects VNI 16777216 (above maximum)", func() {
			cnc := cncWithEVPN("cnc-vni-over", 16777216)
			Expect(k8sClient.Create(ctx, cnc)).NotTo(Succeed())
		})
	})
})
