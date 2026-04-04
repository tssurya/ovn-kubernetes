package vpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vpcv1beta1 "github.com/ovn-kubernetes/ovn-kubernetes/go-controller/pkg/crd/vpc/v1beta1"
)

const (
	ControllerName = "vpc-controller"
	finalizerName  = "vpc.k8s.ovn.org/finalizer"

	nodeSelectionAnnotation = "openshift.io/node-selector"
)

var (
	udnGVR = schema.GroupVersionResource{Group: "k8s.ovn.org", Version: "v1", Resource: "userdefinednetworks"}
	raGVR  = schema.GroupVersionResource{Group: "k8s.ovn.org", Version: "v1", Resource: "routeadvertisements"}
)

// Reconciler reconciles VPC objects by creating/updating/deleting the
// underlying Namespace, UserDefinedNetwork, and RouteAdvertisements resources.
type Reconciler struct {
	client client.Client
}

func NewReconciler(c client.Client) *Reconciler {
	return &Reconciler{client: c}
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	klog.Infof("Reconciling VPC %s", req.Name)
	defer func() {
		klog.Infof("Finished reconciling VPC %s in %v", req.Name, time.Since(startTime))
	}()

	vpc := &vpcv1beta1.VPC{}
	if err := r.client.Get(ctx, req.NamespacedName, vpc); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !vpc.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, vpc)
	}

	if !controllerutil.ContainsFinalizer(vpc, finalizerName) {
		controllerutil.AddFinalizer(vpc, finalizerName)
		if err := r.client.Update(ctx, vpc); err != nil {
			return reconcile.Result{}, err
		}
	}

	return r.reconcileNormal(ctx, vpc)
}

func (r *Reconciler) reconcileNormal(ctx context.Context, vpc *vpcv1beta1.VPC) (reconcile.Result, error) {
	var errs []error

	desiredSubnets := make(map[string]bool)
	for i, subnet := range vpc.Spec.Subnets {
		desiredSubnets[subnetNamespaceName(vpc.Name, subnet.Name)] = true

		if err := r.reconcileSubnet(ctx, vpc, subnet, i); err != nil {
			errs = append(errs, fmt.Errorf("subnet %s: %w", subnet.Name, err))
		}
	}

	if err := r.cleanupOrphanedSubnets(ctx, vpc, desiredSubnets); err != nil {
		errs = append(errs, fmt.Errorf("cleanup orphaned subnets: %w", err))
	}

	readyCond := metav1.Condition{
		Type:               vpcv1beta1.VPCConditionReady,
		ObservedGeneration: vpc.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if len(errs) > 0 {
		readyCond.Status = metav1.ConditionFalse
		readyCond.Reason = "ReconciliationFailed"
		readyCond.Message = fmt.Sprintf("%d subnet(s) failed to reconcile", len(errs))
	} else {
		readyCond.Status = metav1.ConditionTrue
		readyCond.Reason = "ReconciliationSucceeded"
		readyCond.Message = fmt.Sprintf("All %d subnet(s) reconciled", len(vpc.Spec.Subnets))
	}
	meta.SetStatusCondition(&vpc.Status.Conditions, readyCond)
	vpc.Status.SubnetCount = int32(len(vpc.Spec.Subnets))

	if err := r.client.Status().Update(ctx, vpc); err != nil {
		errs = append(errs, fmt.Errorf("status update: %w", err))
	}

	if len(errs) > 0 {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("reconcile errors: %v", errs)
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) reconcileSubnet(ctx context.Context, vpc *vpcv1beta1.VPC, subnet vpcv1beta1.VPCSubnet, subnetIndex int) error {
	nsName := subnetNamespaceName(vpc.Name, subnet.Name)

	if err := r.ensureNamespace(ctx, vpc, subnet, nsName); err != nil {
		return fmt.Errorf("namespace: %w", err)
	}

	if err := r.ensureUDN(ctx, vpc, subnet, nsName, subnetIndex); err != nil {
		return fmt.Errorf("UDN: %w", err)
	}

	if subnet.Type == vpcv1beta1.SubnetTypePublic {
		if err := r.ensureRouteAdvertisements(ctx, vpc, subnet, nsName); err != nil {
			return fmt.Errorf("RouteAdvertisements: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) ensureNamespace(ctx context.Context, vpc *vpcv1beta1.VPC, subnet vpcv1beta1.VPCSubnet, nsName string) error {
	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, types.NamespacedName{Name: nsName}, ns)
	if apierrors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   nsName,
				Labels: subnetLabels(vpc.Name, subnet),
			},
		}
		if subnet.AvailabilityZone != nil && len(subnet.AvailabilityZone.NodeSelector) > 0 {
			ns.Annotations = map[string]string{
				nodeSelectionAnnotation: nodeSelectorString(subnet.AvailabilityZone.NodeSelector),
			}
		}
		klog.Infof("Creating namespace %s for VPC %s subnet %s", nsName, vpc.Name, subnet.Name)
		return r.client.Create(ctx, ns)
	}
	if err != nil {
		return err
	}

	updated := false
	desired := subnetLabels(vpc.Name, subnet)
	for k, v := range desired {
		if ns.Labels[k] != v {
			if ns.Labels == nil {
				ns.Labels = make(map[string]string)
			}
			ns.Labels[k] = v
			updated = true
		}
	}
	if updated {
		return r.client.Update(ctx, ns)
	}
	return nil
}

func (r *Reconciler) ensureUDN(ctx context.Context, vpc *vpcv1beta1.VPC, subnet vpcv1beta1.VPCSubnet, nsName string, subnetIndex int) error {
	cidrs := make([]interface{}, 0, len(subnet.CIDRs))
	for _, c := range subnet.CIDRs {
		cidrs = append(cidrs, string(c))
	}

	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "UserDefinedNetwork",
			"metadata": map[string]interface{}{
				"name":      nsName,
				"namespace": nsName,
			},
			"spec": map[string]interface{}{
				"topology": "Layer2",
				"layer2": map[string]interface{}{
					"role":    "Primary",
					"subnets": cidrs,
				},
				"transport": "EVPN",
				"evpn": map[string]interface{}{
					"vtep": "default",
					"macVRF": map[string]interface{}{
						"vni": int64(100 + subnetIndex),
					},
				},
			},
		},
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "UserDefinedNetwork"})
	err := r.client.Get(ctx, types.NamespacedName{Name: nsName, Namespace: nsName}, existing)
	if apierrors.IsNotFound(err) {
		klog.Infof("Creating UDN %s/%s for VPC %s subnet %s", nsName, nsName, vpc.Name, subnet.Name)
		return r.client.Create(ctx, desired)
	}
	return err
}

func (r *Reconciler) ensureRouteAdvertisements(ctx context.Context, vpc *vpcv1beta1.VPC, subnet vpcv1beta1.VPCSubnet, nsName string) error {
	raName := fmt.Sprintf("%s-ra", nsName)

	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.ovn.org/v1",
			"kind":       "RouteAdvertisements",
			"metadata": map[string]interface{}{
				"name": raName,
			},
			"spec": map[string]interface{}{
				"networkSelectors": []interface{}{
					map[string]interface{}{
						"networkSelectionType": "PrimaryUserDefinedNetworks",
						"primaryUserDefinedNetworkSelector": map[string]interface{}{
							"namespaceSelector": map[string]interface{}{
								"matchLabels": map[string]interface{}{
									vpcv1beta1.LabelVPCName:   vpc.Name,
									vpcv1beta1.LabelVPCSubnet: subnet.Name,
								},
							},
						},
					},
				},
				"nodeSelector":             map[string]interface{}{},
				"frrConfigurationSelector": map[string]interface{}{},
				"advertisements":           []interface{}{"PodNetwork"},
			},
		},
	}

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "RouteAdvertisements"})
	err := r.client.Get(ctx, types.NamespacedName{Name: raName}, existing)
	if apierrors.IsNotFound(err) {
		klog.Infof("Creating RouteAdvertisements %s for VPC %s public subnet %s", raName, vpc.Name, subnet.Name)
		return r.client.Create(ctx, desired)
	}
	return err
}

func (r *Reconciler) reconcileDelete(ctx context.Context, vpc *vpcv1beta1.VPC) (reconcile.Result, error) {
	klog.Infof("Handling deletion for VPC %s", vpc.Name)

	nsList := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nsList, client.MatchingLabels{vpcv1beta1.LabelVPCName: vpc.Name}); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing namespaces: %w", err)
	}

	for i := range nsList.Items {
		ns := &nsList.Items[i]
		udnName := ns.Name
		if err := r.deleteUDN(ctx, udnName, ns.Name); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("deleting UDN %s: %w", udnName, err)
		}

		raName := fmt.Sprintf("%s-ra", ns.Name)
		if err := r.deleteRA(ctx, raName); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("deleting RA %s: %w", raName, err)
		}

		if err := r.client.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("deleting namespace %s: %w", ns.Name, err)
		}
	}

	controllerutil.RemoveFinalizer(vpc, finalizerName)
	if err := r.client.Update(ctx, vpc); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) cleanupOrphanedSubnets(ctx context.Context, vpc *vpcv1beta1.VPC, desired map[string]bool) error {
	nsList := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nsList, client.MatchingLabels{vpcv1beta1.LabelVPCName: vpc.Name}); err != nil {
		return err
	}

	for i := range nsList.Items {
		ns := &nsList.Items[i]
		if desired[ns.Name] {
			continue
		}
		klog.Infof("Cleaning up orphaned namespace %s from VPC %s", ns.Name, vpc.Name)
		if err := r.deleteUDN(ctx, ns.Name, ns.Name); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		raName := fmt.Sprintf("%s-ra", ns.Name)
		if err := r.deleteRA(ctx, raName); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err := r.client.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *Reconciler) deleteUDN(ctx context.Context, name, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "UserDefinedNetwork"})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return r.client.Delete(ctx, obj)
}

func (r *Reconciler) deleteRA(ctx context.Context, name string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "k8s.ovn.org", Version: "v1", Kind: "RouteAdvertisements"})
	obj.SetName(name)
	return r.client.Delete(ctx, obj)
}

// SetupWithManager wires the reconciler into the controller-runtime manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vpcv1beta1.VPC{}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}

func subnetNamespaceName(vpcName, subnetName string) string {
	return fmt.Sprintf("%s-%s", vpcName, subnetName)
}

func subnetLabels(vpcName string, subnet vpcv1beta1.VPCSubnet) map[string]string {
	t := subnet.Type
	if t == "" {
		t = vpcv1beta1.SubnetTypePrivate
	}
	return map[string]string{
		vpcv1beta1.LabelVPCName:                    vpcName,
		vpcv1beta1.LabelVPCSubnet:                  subnet.Name,
		vpcv1beta1.LabelVPCType:                    string(t),
		"k8s.ovn.org/primary-user-defined-network": "",
	}
}

func nodeSelectorString(sel map[string]string) string {
	parts := make([]string, 0, len(sel))
	for k, v := range sel {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}
