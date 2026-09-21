// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/types"

// EVPNConfig contains configuration options for networks operating in EVPN mode.
// +kubebuilder:validation:XValidation:rule="has(self.macVRF) || has(self.ipVRF)", message="at least one of macVRF or ipVRF must be specified"
// +kubebuilder:validation:XValidation:rule="!has(self.macVRF) || !has(self.ipVRF) || self.macVRF.vni != self.ipVRF.vni", message="macVRF and ipVRF must use different VNIs"
type EVPNConfig struct {
	// VTEP is the name of the VTEP CR that defines VTEP IPs for EVPN.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +required
	VTEP string `json:"vtep"`

	// MACVRF contains the MAC-VRF configuration for Layer 2 EVPN.
	// This field is required for Layer2 topology and forbidden for Layer3 topology.
	// +optional
	MACVRF *types.VRFConfig `json:"macVRF,omitempty"`

	// IPVRF contains the IP-VRF configuration for Layer 3 EVPN.
	// This field is required for Layer3 topology and optional for Layer2 topology.
	// +optional
	IPVRF *types.VRFConfig `json:"ipVRF,omitempty"`
}
