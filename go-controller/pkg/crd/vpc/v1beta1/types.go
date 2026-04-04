package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	SubnetTypePublic   SubnetType = "Public"
	SubnetTypePrivate  SubnetType = "Private"
	SubnetTypeIsolated SubnetType = "Isolated"
	SubnetTypeVPNOnly  SubnetType = "VPNOnly"
)

// VPC defines a Virtual Private Cloud — a logically isolated virtual network.
// The VPC controller watches VPC resources and reconciles underlying
// OVN-Kubernetes objects (Namespaces, UserDefinedNetworks, RouteAdvertisements).
//
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=vpcs,scope=Cluster,shortName=vpc
// +kubebuilder:singular=vpc
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Subnets",type=integer,JSONPath=".status.subnetCount"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"
type VPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	// +required
	Spec VPCSpec `json:"spec"`

	// +optional
	Status VPCStatus `json:"status,omitempty"`
}

// VPCSpec defines the desired state of a VPC.
type VPCSpec struct {
	// Subnets is the list of subnets within this VPC.
	// Each subnet maps 1:1 to a Namespace + UserDefinedNetwork (L2 EVPN).
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +listType=map
	// +listMapKey=name
	// +required
	Subnets []VPCSubnet `json:"subnets"`
}

// SubnetType defines the connectivity type for a VPC subnet.
// +kubebuilder:validation:Enum=Public;Private;Isolated;VPNOnly
type SubnetType string

// VPCSubnet describes a single subnet within a VPC.
type VPCSubnet struct {
	// Name is the subnet identifier, unique within the VPC.
	// The resulting Namespace and UDN are named "<vpc-name>-<subnet-name>".
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +required
	Name string `json:"name"`

	// CIDRs are the IP ranges for this subnet.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=2
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CIDRs are immutable once set"
	// +required
	CIDRs []CIDR `json:"cidrs"`

	// Type is the connectivity type. Defaults to Private.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable once set"
	// +kubebuilder:default=Private
	// +optional
	Type SubnetType `json:"type,omitempty"`

	// AvailabilityZone optionally pins pods to specific nodes/clusters.
	// +optional
	AvailabilityZone *AvailabilityZone `json:"availabilityZone,omitempty"`
}

// CIDR is a network address in CIDR notation (e.g. "10.128.0.0/16", "fd00::/48").
// +kubebuilder:validation:XValidation:rule="isCIDR(self)",message="CIDR is invalid"
// +kubebuilder:validation:MaxLength=43
type CIDR string

// AvailabilityZone pins workloads to specific nodes or clusters.
type AvailabilityZone struct {
	// ClusterSelector selects the target cluster(s) for multi-cluster deployments.
	// +required
	ClusterSelector metav1.LabelSelector `json:"clusterSelector"`

	// NodeSelector restricts the subnet's pods to nodes matching these labels.
	// When set, the controller annotates the namespace with openshift.io/node-selector.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// VPCStatus describes the observed state of a VPC.
type VPCStatus struct {
	// SubnetCount is the number of subnets in this VPC (for printer column).
	// +optional
	SubnetCount int32 `json:"subnetCount,omitempty"`

	// Conditions represent the latest observations of the VPC's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// VPCList contains a list of VPCs.
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VPC `json:"items"`
}

// Condition types for VPC status.
const (
	// VPCConditionReady indicates all subnets have been reconciled successfully.
	VPCConditionReady = "Ready"

	// VPCConditionDegraded indicates some subnets failed to reconcile.
	VPCConditionDegraded = "Degraded"
)

// Well-known labels applied to VPC-managed namespaces.
const (
	LabelVPCName   = "vpc.k8s.ovn.org/name"
	LabelVPCSubnet = "vpc.k8s.ovn.org/subnet"
	LabelVPCType   = "vpc.k8s.ovn.org/type"
)
