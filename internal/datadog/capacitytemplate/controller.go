package capacitytemplate

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ManagedByLabel = "storage.datadoghq.com/managed-by"
	ManagedByValue = "datadog-topolvm-capacity-template-controller"

	StorageClassAnnotation       = "storage.datadoghq.com/storage-class"
	DeviceClassAnnotation        = "storage.datadoghq.com/device-class"
	NodeGroupNamespaceAnnotation = "storage.datadoghq.com/nodegroup-namespace"
	NodeGroupNameAnnotation      = "storage.datadoghq.com/nodegroup-name"
	NodeGroupUIDAnnotation       = "storage.datadoghq.com/nodegroup-uid"

	TemplateNodeLabel       = "cluster-autoscaler.kubernetes.io/template-node"
	NodeGroupNamespaceLabel = "nodegroups.datadoghq.com/namespace"
	NodeGroupNameLabel      = "nodegroups.datadoghq.com/name"

	topolvmProvisioner   = "topolvm.io"
	deviceClassParameter = "topolvm.io/device-class"
)

var (
	NodeGroupGVK     = schema.GroupVersionKind{Group: "datadoghq.com", Version: "v1", Kind: "NodeGroup"}
	NodeGroupListGVK = schema.GroupVersionKind{Group: "datadoghq.com", Version: "v1", Kind: "NodeGroupList"}
)

// AddNodeGroupToScheme registers the downstream NodeGroup API without taking a
// dependency on a generated NodeGroup client.
func AddNodeGroupToScheme(scheme *runtime.Scheme) {
	scheme.AddKnownTypeWithName(NodeGroupGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(NodeGroupListGVK, &unstructured.UnstructuredList{})
	metav1.AddToGroupVersion(scheme, NodeGroupGVK.GroupVersion())
}

// Reconciler publishes one template-only capacity per NodeGroup for each
// configured template whose StorageClass is compatible and whose handler
// reports capacity for that NodeGroup.
type Reconciler struct {
	client         client.Client
	recorder       events.EventRecorder
	templates      []templateBinding
	storageClasses map[string]struct{}
}

// templateBinding is a validated template with its handler resolved once at
// construction time, so reconciliation never dispatches on an unknown name.
type templateBinding struct {
	TemplateConfig
	handler CapacityHandler
}

// NewReconciler validates config against the implemented capacity handlers
// and returns a reconciler for all configured templates.
func NewReconciler(c client.Client, recorder events.EventRecorder, config Config) (*Reconciler, error) {
	return newReconciler(c, recorder, config, defaultCapacityHandlers())
}

func newReconciler(
	c client.Client,
	recorder events.EventRecorder,
	config Config,
	handlers map[string]CapacityHandler,
) (*Reconciler, error) {
	if err := config.validate(handlers); err != nil {
		return nil, err
	}
	r := &Reconciler{
		client:         c,
		recorder:       recorder,
		templates:      make([]templateBinding, 0, len(config.Templates)),
		storageClasses: make(map[string]struct{}, len(config.Templates)),
	}
	for _, template := range config.Templates {
		r.templates = append(r.templates, templateBinding{
			TemplateConfig: template,
			handler:        handlers[template.Handler],
		})
		r.storageClasses[template.StorageClass] = struct{}{}
	}
	return r, nil
}

// Reconcile reconciles every configured template for a single NodeGroup.
// Templates are independent: a failure in one template does not prevent the
// others from converging, and errors are aggregated for retry.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ng := newNodeGroup()
	if err := r.client.Get(ctx, req.NamespacedName, ng); err != nil {
		if apierrors.IsNotFound(err) {
			// Managed capacities have same-namespace owner references, so the
			// Kubernetes garbage collector handles NodeGroup deletion.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := validateNodeGroupIdentity(ng); err != nil {
		r.warning(ng, "InvalidTopology", err)
		return ctrl.Result{}, r.deleteStaleCapacities(ctx, ng, nil)
	}

	keep := make(map[string]struct{}, len(r.templates))
	var errs []error
	for _, template := range r.templates {
		name, err := r.reconcileTemplate(ctx, ng, template)
		if name != "" {
			keep[name] = struct{}{}
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("StorageClass %q: %w", template.StorageClass, err))
		}
	}
	if err := r.deleteStaleCapacities(ctx, ng, keep); err != nil {
		errs = append(errs, err)
	}
	return ctrl.Result{}, errors.Join(errs...)
}

func validateNodeGroupIdentity(ng *unstructured.Unstructured) error {
	if errs := validation.IsValidLabelValue(ng.GetNamespace()); len(errs) != 0 {
		return fmt.Errorf("NodeGroup namespace cannot be represented by a topology label: %v", errs)
	}
	if errs := validation.IsValidLabelValue(ng.GetName()); len(errs) != 0 {
		return fmt.Errorf("NodeGroup name cannot be represented by a topology label: %v", errs)
	}
	if ng.GetUID() == "" {
		return fmt.Errorf("NodeGroup has no UID")
	}
	return nil
}

// reconcileTemplate converges the capacity for one template and returns the
// name of the capacity that must survive stale cleanup, or "" if this template
// should currently have no capacity for the NodeGroup.
func (r *Reconciler) reconcileTemplate(
	ctx context.Context,
	ng *unstructured.Unstructured,
	template templateBinding,
) (string, error) {
	name := capacityNameFor(ng, template.TemplateConfig)

	compatible, err := r.storageClassCompatible(ctx, template.TemplateConfig)
	if err != nil {
		// Compatibility is unknown, so leave any existing object in place and
		// retry rather than flapping it on a transient API error.
		return name, err
	}
	if !compatible {
		return "", nil
	}

	capacity, err := template.handler.CalculateCapacity(ctx, ng, template.SpareGB)
	if err != nil {
		// Invalid NodeGroup storage is not retryable; fail closed for this
		// template only.
		r.warning(ng, "InvalidStorage", fmt.Errorf(
			"StorageClass %q (handler %q): %w", template.StorageClass, template.Handler, err))
		return "", nil
	}
	if capacity == nil {
		return "", nil
	}

	return name, r.applyCapacity(ctx, ng, r.desiredCapacity(ng, template.TemplateConfig, capacity))
}

func (r *Reconciler) applyCapacity(
	ctx context.Context,
	ng *unstructured.Unstructured,
	desired *storagev1.CSIStorageCapacity,
) error {
	existing := &storagev1.CSIStorageCapacity{}
	key := types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name}
	if err := r.client.Get(ctx, key, existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if err := r.client.Create(ctx, desired); err != nil {
			if apierrors.IsAlreadyExists(err) {
				r.warning(ng, "NameCollision", fmt.Errorf("desired CSIStorageCapacity %s is already occupied", key.String()))
			}
			return err
		}
		return nil
	}

	if !managedCapacityForNodeGroup(existing, ng) {
		err := fmt.Errorf("desired CSIStorageCapacity name %s is occupied by an unmanaged or inconsistent object", key.String())
		r.warning(ng, "NameCollision", err)
		return err
	}

	updated := existing.DeepCopy()
	applyDesired(updated, desired)
	if reflect.DeepEqual(existing, updated) {
		return nil
	}
	return r.client.Patch(ctx, updated, client.MergeFrom(existing))
}

// storageClassCompatible reports whether the template's StorageClass exists
// and is a delayed-binding TopoLVM StorageClass for the template's device
// class.
func (r *Reconciler) storageClassCompatible(ctx context.Context, template TemplateConfig) (bool, error) {
	sc := &storagev1.StorageClass{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: template.StorageClass}, sc); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return sc.Provisioner == topolvmProvisioner &&
		sc.VolumeBindingMode != nil && *sc.VolumeBindingMode == storagev1.VolumeBindingWaitForFirstConsumer &&
		sc.Parameters[deviceClassParameter] == template.DeviceClass, nil
}

func capacityNameFor(ng *unstructured.Unstructured, template TemplateConfig) string {
	return CapacityName(ng.GetNamespace(), ng.GetName(), template.StorageClass, template.DeviceClass)
}

func (r *Reconciler) desiredCapacity(
	ng *unstructured.Unstructured,
	template TemplateConfig,
	capacity *resource.Quantity,
) *storagev1.CSIStorageCapacity {
	q := capacity.DeepCopy()
	max := q.DeepCopy()
	controller := true
	return &storagev1.CSIStorageCapacity{
		TypeMeta: metav1.TypeMeta{APIVersion: storagev1.SchemeGroupVersion.String(), Kind: "CSIStorageCapacity"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      capacityNameFor(ng, template),
			Namespace: ng.GetNamespace(),
			Labels:    map[string]string{ManagedByLabel: ManagedByValue},
			Annotations: map[string]string{
				StorageClassAnnotation:       template.StorageClass,
				DeviceClassAnnotation:        template.DeviceClass,
				NodeGroupNamespaceAnnotation: ng.GetNamespace(),
				NodeGroupNameAnnotation:      ng.GetName(),
				NodeGroupUIDAnnotation:       string(ng.GetUID()),
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: NodeGroupGVK.GroupVersion().String(),
				Kind:       NodeGroupGVK.Kind,
				Name:       ng.GetName(),
				UID:        ng.GetUID(),
				Controller: &controller,
			}},
		},
		StorageClassName: template.StorageClass,
		NodeTopology: &metav1.LabelSelector{MatchLabels: map[string]string{
			TemplateNodeLabel:       "true",
			NodeGroupNamespaceLabel: ng.GetNamespace(),
			NodeGroupNameLabel:      ng.GetName(),
		}},
		Capacity:          &q,
		MaximumVolumeSize: &max,
	}
}

func applyDesired(current, desired *storagev1.CSIStorageCapacity) {
	if current.Labels == nil {
		current.Labels = make(map[string]string)
	}
	current.Labels[ManagedByLabel] = ManagedByValue
	if current.Annotations == nil {
		current.Annotations = make(map[string]string)
	}
	for _, key := range []string{
		StorageClassAnnotation,
		DeviceClassAnnotation,
		NodeGroupNamespaceAnnotation,
		NodeGroupNameAnnotation,
		NodeGroupUIDAnnotation,
	} {
		current.Annotations[key] = desired.Annotations[key]
	}
	current.OwnerReferences = append([]metav1.OwnerReference(nil), desired.OwnerReferences...)
	current.StorageClassName = desired.StorageClassName
	current.NodeTopology = desired.NodeTopology.DeepCopy()
	capacity := desired.Capacity.DeepCopy()
	maximumVolumeSize := desired.MaximumVolumeSize.DeepCopy()
	current.Capacity = &capacity
	current.MaximumVolumeSize = &maximumVolumeSize
}

// deleteStaleCapacities removes controller-owned capacities for this NodeGroup
// whose names are not in keep. Ownership detection deliberately does not depend
// on the current template configuration, so removing a template or changing
// its StorageClass/device class cleans up objects created by an earlier
// process. A nil keep set deletes every managed capacity for the NodeGroup.
func (r *Reconciler) deleteStaleCapacities(
	ctx context.Context,
	ng *unstructured.Unstructured,
	keep map[string]struct{},
) error {
	capacities := &storagev1.CSIStorageCapacityList{}
	if err := r.client.List(
		ctx,
		capacities,
		client.InNamespace(ng.GetNamespace()),
		client.MatchingLabels{ManagedByLabel: ManagedByValue},
	); err != nil {
		return err
	}

	for i := range capacities.Items {
		capacity := &capacities.Items[i]
		if _, kept := keep[capacity.Name]; kept || !managedCapacityForNodeGroup(capacity, ng) {
			continue
		}
		if err := r.client.Delete(ctx, capacity); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete stale CSIStorageCapacity %s/%s: %w", capacity.Namespace, capacity.Name, err)
		}
	}
	return nil
}

// managedCapacityForNodeGroup verifies ownership without relying on the
// controller's current template configuration.
func managedCapacityForNodeGroup(
	capacity *storagev1.CSIStorageCapacity,
	ng *unstructured.Unstructured,
) bool {
	if capacity.Labels[ManagedByLabel] != ManagedByValue || capacity.Namespace != ng.GetNamespace() {
		return false
	}
	annotations := capacity.Annotations
	if annotations[StorageClassAnnotation] == "" ||
		annotations[NodeGroupNamespaceAnnotation] != ng.GetNamespace() ||
		annotations[NodeGroupNameAnnotation] != ng.GetName() ||
		annotations[NodeGroupUIDAnnotation] == "" {
		return false
	}
	owner := metav1.GetControllerOf(capacity)
	return owner != nil &&
		owner.APIVersion == NodeGroupGVK.GroupVersion().String() &&
		owner.Kind == NodeGroupGVK.Kind &&
		owner.Name == ng.GetName() &&
		string(owner.UID) == annotations[NodeGroupUIDAnnotation]
}

func (r *Reconciler) warning(ng *unstructured.Unstructured, reason string, err error) {
	if r.recorder != nil {
		r.recorder.Eventf(
			ng,
			nil,
			corev1.EventTypeWarning,
			reason,
			"ReconcileTemplateCapacity",
			"%v",
			err,
		)
	}
}

// SetupWithManager watches NodeGroups, their owned capacities, and every
// configured StorageClass. A configured StorageClass change enqueues every
// NodeGroup because compatibility determines whether that template's capacity
// may exist for any NodeGroup.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("datadog-topolvm-capacity-template-controller").
		For(newNodeGroup()).
		Owns(&storagev1.CSIStorageCapacity{}).
		Watches(
			&storagev1.StorageClass{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForStorageClass),
		).
		Complete(r)
}

func (r *Reconciler) requestsForStorageClass(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	if _, configured := r.storageClasses[object.GetName()]; !configured {
		return nil
	}

	nodeGroups := newNodeGroupList()
	if err := r.client.List(ctx, nodeGroups); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to list NodeGroups after StorageClass change")
		return nil
	}
	requests := make([]reconcile.Request, 0, len(nodeGroups.Items))
	for i := range nodeGroups.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: nodeGroups.Items[i].GetNamespace(),
			Name:      nodeGroups.Items[i].GetName(),
		}})
	}
	return requests
}

func newNodeGroup() *unstructured.Unstructured {
	ng := &unstructured.Unstructured{}
	ng.SetGroupVersionKind(NodeGroupGVK)
	return ng
}

func newNodeGroupList() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(NodeGroupListGVK)
	return list
}
