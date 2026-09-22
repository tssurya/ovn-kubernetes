// SPDX-FileCopyrightText: Copyright The OVN-Kubernetes Contributors
// SPDX-License-Identifier: Apache-2.0

// Package evpnregistry provides a cluster-wide per-VTEP VNI conflict-detection
// registry for EVPN networks. VNIs are user-specified in CRD specs; the registry
// ensures no two resources sharing the same VTEP claim the same VNI.
package evpnregistry

import (
	"errors"
	"fmt"
	"sync"
)

// vniKey identifies a VNI within a VTEP scope. VNIs must be unique per VTEP.
type vniKey struct {
	vtep string
	vni  int32
}

// Registry is a thread-safe cluster-wide VNI conflict-detection registry.
// It maps (VTEP, VNI) → owner name to ensure no two networks sharing the same
// VTEP use the same VNI. Shared by both the UDN controller (for CUDN VNIs)
// and the CNC controller (for parent VRF VNIs).
type Registry struct {
	mu       sync.RWMutex
	reserved map[vniKey]string // {vtep, vni} → owner name
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		reserved: make(map[vniKey]string),
	}
}

// Reserve registers one or more VNIs for owner on the given VTEP.
// VNI 0 is silently skipped (used as "not set" sentinel).
// If any VNI is already owned by a different owner on the same VTEP, that
// conflict is collected and returned as an aggregate error; remaining VNIs
// are still registered. Re-reserving the same owner+VNI is a no-op.
func (r *Registry) Reserve(owner, vtep string, vnis ...int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for _, vni := range vnis {
		if vni == 0 {
			continue
		}
		key := vniKey{vtep: vtep, vni: vni}
		if existing, ok := r.reserved[key]; ok && existing != owner {
			errs = append(errs, fmt.Errorf("VNI %d on VTEP %q is already reserved by network %q", vni, vtep, existing))
			continue
		}
		r.reserved[key] = owner
	}
	return errors.Join(errs...)
}

// Release removes all VNI reservations owned by the given owner.
func (r *Registry) Release(owner string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, o := range r.reserved {
		if o == owner {
			delete(r.reserved, key)
		}
	}
}
