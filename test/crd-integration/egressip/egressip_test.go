// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

package egressip

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	egressipv1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/egressip/v1"
)

var _ = Describe("EgressIP CRD", func() {
	Context("spec.egressNodeSelector defaulting", func() {
		// The +kubebuilder:default marker on EgressNodeSelector means the API
		// server fills in the egress-assignable default when the field is
		// omitted.  EgressNodeSelector is a value-type metav1.LabelSelector with
		// the `omitzero` json tag: the zero-value struct is dropped from the
		// wire, so leaving the field at its zero value is what triggers the CRD
		// default.  Note there is no nil here — a zero-value struct and an
		// "explicit empty {}" are the same value, so they cannot be
		// distinguished on the wire (see the explicit-empty cases below).

		It("fills in the egress-assignable default when egressNodeSelector is omitted", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-default-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.1"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					// EgressNodeSelector left at its zero value — with omitzero it
					// is dropped from the wire, so the API server injects the default:
					// {matchExpressions: [{key: k8s.ovn.org/egress-assignable, operator: Exists}]}
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })

			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(ConsistOf(
				metav1.LabelSelectorRequirement{
					Key:      "k8s.ovn.org/egress-assignable",
					Operator: metav1.LabelSelectorOpExists,
				},
			))
			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(BeEmpty())
		})

		It("preserves a custom egressNodeSelector when one is explicitly provided", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-custom-selector-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.2"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					EgressNodeSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"pool": "egress"},
					},
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })

			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(Equal(map[string]string{"pool": "egress"}))
			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(BeEmpty())
		})

		It("accepts an explicit empty selector ({}) that matches all nodes", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-empty-selector-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.3"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					// "Explicit" empty selector — intended to match all nodes.
					// With a value type this is the zero value, so omitzero drops
					// it from the wire and it is indistinguishable from "unset".
					// EXPECTATION (pointer design): preserved as empty.
					// LIKELY under omitzero: the API server defaults it instead.
					EgressNodeSelector: metav1.LabelSelector{},
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })

			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(BeEmpty())
			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(BeEmpty())
		})
	})

	Context("spec.egressNodeSelector updates", func() {
		// These lock down the mutation semantics of the value-type + omitzero
		// default design. CRD structural-schema defaulting runs on every write,
		// updates included. Because EgressNodeSelector is a metav1.LabelSelector
		// with omitzero, a zero-value struct is dropped from the PUT body, so the
		// API server re-injects the default on update — there is no way to clear
		// the selector back to empty.

		It("re-applies the default when the selector is reset to zero on update", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-update-unset-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.10"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					EgressNodeSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"pool": "egress"},
					},
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })
			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(Equal(map[string]string{"pool": "egress"}))

			// Reset to the zero value, then update: omitzero drops it from the
			// PUT body, so the API server re-injects the egress-assignable default.
			eip.Spec.EgressNodeSelector = metav1.LabelSelector{}
			Expect(k8sClient.Update(ctx, eip)).To(Succeed())

			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(ConsistOf(
				metav1.LabelSelectorRequirement{
					Key:      "k8s.ovn.org/egress-assignable",
					Operator: metav1.LabelSelectorOpExists,
				},
			))
			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(BeEmpty())
		})

		It("preserves an explicit empty selector ({}) set on update", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-update-empty-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.11"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					EgressNodeSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"pool": "egress"},
					},
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })

			// Reset to an empty selector on update. Under the pointer design a
			// non-nil {} survived untouched; under omitzero the zero value is
			// dropped from the PUT body, so the default LIKELY overwrites it.
			eip.Spec.EgressNodeSelector = metav1.LabelSelector{}
			Expect(k8sClient.Update(ctx, eip)).To(Succeed())

			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(BeEmpty())
			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(BeEmpty())
		})

		It("allows switching from the default to a custom selector on update", func() {
			eip := &egressipv1.EgressIP{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-update-custom-",
				},
				Spec: egressipv1.EgressIPSpec{
					EgressIPs: []string{"192.0.2.12"},
					NamespaceSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "default"},
					},
					// Omitted — starts life with the injected default.
				},
				Status: egressipv1.EgressIPStatus{Items: []egressipv1.EgressIPStatusItem{}},
			}
			Expect(k8sClient.Create(ctx, eip)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, eip) })
			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(ConsistOf(
				metav1.LabelSelectorRequirement{
					Key:      "k8s.ovn.org/egress-assignable",
					Operator: metav1.LabelSelectorOpExists,
				},
			))

			eip.Spec.EgressNodeSelector = metav1.LabelSelector{
				MatchLabels: map[string]string{"pool": "egress"},
			}
			Expect(k8sClient.Update(ctx, eip)).To(Succeed())

			Expect(eip.Spec.EgressNodeSelector.MatchLabels).To(Equal(map[string]string{"pool": "egress"}))
			Expect(eip.Spec.EgressNodeSelector.MatchExpressions).To(BeEmpty())
		})
	})
})
