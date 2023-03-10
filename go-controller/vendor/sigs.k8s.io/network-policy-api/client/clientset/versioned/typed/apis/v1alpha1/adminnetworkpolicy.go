/*
Copyright 2020 The Kubernetes Authors.

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
// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "sigs.k8s.io/network-policy-api/apis/v1alpha1"
	scheme "sigs.k8s.io/network-policy-api/client/clientset/versioned/scheme"
)

// AdminNetworkPoliciesGetter has a method to return a AdminNetworkPolicyInterface.
// A group's client should implement this interface.
type AdminNetworkPoliciesGetter interface {
	AdminNetworkPolicies(namespace string) AdminNetworkPolicyInterface
}

// AdminNetworkPolicyInterface has methods to work with AdminNetworkPolicy resources.
type AdminNetworkPolicyInterface interface {
	Create(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.CreateOptions) (*v1alpha1.AdminNetworkPolicy, error)
	Update(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.UpdateOptions) (*v1alpha1.AdminNetworkPolicy, error)
	UpdateStatus(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.UpdateOptions) (*v1alpha1.AdminNetworkPolicy, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.AdminNetworkPolicy, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.AdminNetworkPolicyList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AdminNetworkPolicy, err error)
	AdminNetworkPolicyExpansion
}

// adminNetworkPolicies implements AdminNetworkPolicyInterface
type adminNetworkPolicies struct {
	client rest.Interface
	ns     string
}

// newAdminNetworkPolicies returns a AdminNetworkPolicies
func newAdminNetworkPolicies(c *PolicyV1alpha1Client, namespace string) *adminNetworkPolicies {
	return &adminNetworkPolicies{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the adminNetworkPolicy, and returns the corresponding adminNetworkPolicy object, and an error if there is any.
func (c *adminNetworkPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.AdminNetworkPolicy, err error) {
	result = &v1alpha1.AdminNetworkPolicy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AdminNetworkPolicies that match those selectors.
func (c *adminNetworkPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.AdminNetworkPolicyList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.AdminNetworkPolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested adminNetworkPolicies.
func (c *adminNetworkPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a adminNetworkPolicy and creates it.  Returns the server's representation of the adminNetworkPolicy, and an error, if there is any.
func (c *adminNetworkPolicies) Create(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.CreateOptions) (result *v1alpha1.AdminNetworkPolicy, err error) {
	result = &v1alpha1.AdminNetworkPolicy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(adminNetworkPolicy).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a adminNetworkPolicy and updates it. Returns the server's representation of the adminNetworkPolicy, and an error, if there is any.
func (c *adminNetworkPolicies) Update(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.UpdateOptions) (result *v1alpha1.AdminNetworkPolicy, err error) {
	result = &v1alpha1.AdminNetworkPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		Name(adminNetworkPolicy.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(adminNetworkPolicy).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *adminNetworkPolicies) UpdateStatus(ctx context.Context, adminNetworkPolicy *v1alpha1.AdminNetworkPolicy, opts v1.UpdateOptions) (result *v1alpha1.AdminNetworkPolicy, err error) {
	result = &v1alpha1.AdminNetworkPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		Name(adminNetworkPolicy.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(adminNetworkPolicy).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the adminNetworkPolicy and deletes it. Returns an error if one occurs.
func (c *adminNetworkPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *adminNetworkPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched adminNetworkPolicy.
func (c *adminNetworkPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.AdminNetworkPolicy, err error) {
	result = &v1alpha1.AdminNetworkPolicy{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("adminnetworkpolicies").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
