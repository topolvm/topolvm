package capacitytemplate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const metadataKeepValue = "yes"

var (
	remoteTemplate = TemplateConfig{
		StorageClass: "ephemeral-remote-data",
		DeviceClass:  "remote-ssd",
		SpareGB:      1,
		Handler:      HandlerRemoteLVM,
	}
	// remoteXFSTemplate shares the remote device class through a second
	// StorageClass, which is the multi-template shape available today with
	// only the remote-lvm handler implemented.
	remoteXFSTemplate = TemplateConfig{
		StorageClass: "ephemeral-remote-data-xfs",
		DeviceClass:  "remote-ssd",
		SpareGB:      2,
		Handler:      HandlerRemoteLVM,
	}
	testConfig  = Config{Templates: []TemplateConfig{remoteTemplate}}
	multiConfig = Config{Templates: []TemplateConfig{remoteTemplate, remoteXFSTemplate}}
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	AddNodeGroupToScheme(scheme)
	return scheme
}

func testNodeGroup(name, uid string, volumes ...interface{}) *unstructured.Unstructured {
	ng := nodeGroupWithVolumes(volumes...)
	ng.SetNamespace("datadog")
	ng.SetName(name)
	ng.SetUID(types.UID(uid))
	return ng
}

func mustReconciler(t *testing.T, c client.Client, config Config) *Reconciler {
	t.Helper()
	r, err := NewReconciler(c, nil, config)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type staticCapacityHandler struct {
	capacity *resource.Quantity
	err      error
}

func (h staticCapacityHandler) CalculateCapacity(
	_ context.Context,
	_ *unstructured.Unstructured,
	_ int64,
) (*resource.Quantity, error) {
	if h.err != nil || h.capacity == nil {
		return nil, h.err
	}
	capacity := h.capacity.DeepCopy()
	return &capacity, nil
}

func quantity(value string) *resource.Quantity {
	q := resource.MustParse(value)
	return &q
}

func compatibleStorageClass(template TemplateConfig) *storagev1.StorageClass {
	mode := storagev1.VolumeBindingWaitForFirstConsumer
	return &storagev1.StorageClass{
		ObjectMeta:        metav1.ObjectMeta{Name: template.StorageClass},
		Provisioner:       topolvmProvisioner,
		VolumeBindingMode: &mode,
		Parameters:        map[string]string{deviceClassParameter: template.DeviceClass},
	}
}

func reconcileOnce(t *testing.T, r *Reconciler, name string) error {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "datadog", Name: name}})
	return err
}

func capacityKey(ng *unstructured.Unstructured, template TemplateConfig) types.NamespacedName {
	return types.NamespacedName{Namespace: ng.GetNamespace(), Name: capacityNameFor(ng, template)}
}

func getCapacity(t *testing.T, c client.Client, key types.NamespacedName) *storagev1.CSIStorageCapacity {
	t.Helper()
	got := &storagev1.CSIStorageCapacity{}
	if err := c.Get(context.Background(), key, got); err != nil {
		t.Fatalf("get CSIStorageCapacity %s: %v", key, err)
	}
	return got
}

func assertCapacityAbsent(t *testing.T, c client.Client, key types.NamespacedName) {
	t.Helper()
	err := c.Get(context.Background(), key, &storagev1.CSIStorageCapacity{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("CSIStorageCapacity %s: got err %v, want NotFound", key, err)
	}
}

func managedCapacityNames(t *testing.T, c client.Client) map[string]bool {
	t.Helper()
	list := &storagev1.CSIStorageCapacityList{}
	if err := c.List(context.Background(), list, client.MatchingLabels{ManagedByLabel: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(list.Items))
	for i := range list.Items {
		names[list.Items[i].Name] = true
	}
	return names
}

// Config validation cases are covered by TestConfigValidate; this only checks
// that construction enforces them.
func TestNewReconcilerRejectsInvalidConfig(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()
	unsupported := remoteXFSTemplate
	unsupported.Handler = "local-lvm"
	if _, err := NewReconciler(c, nil, Config{Templates: []TemplateConfig{remoteTemplate, unsupported}}); err == nil {
		t.Fatal("NewReconciler() accepted an unsupported handler")
	}
}

func TestReconcileCreatesCapacity(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("remote-workers", "uid-1", remoteVolume("data", "lvm", "50Gi"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ng, compatibleStorageClass(remoteTemplate)).Build()
	r := mustReconciler(t, c, testConfig)

	if err := reconcileOnce(t, r, "remote-workers"); err != nil {
		t.Fatal(err)
	}
	got := getCapacity(t, c, capacityKey(ng, remoteTemplate))

	wantBytes := int64(49*(1<<30) - 8*(1<<20))
	if got.Capacity == nil || got.Capacity.Value() != wantBytes {
		t.Fatalf("capacity = %v, want %d", got.Capacity, wantBytes)
	}
	if got.MaximumVolumeSize == nil || got.MaximumVolumeSize.Value() != wantBytes {
		t.Fatalf("maximumVolumeSize = %v, want %d", got.MaximumVolumeSize, wantBytes)
	}
	if got.StorageClassName != remoteTemplate.StorageClass || got.Labels[ManagedByLabel] != ManagedByValue {
		t.Fatalf("unexpected storage class or labels: %#v", got)
	}
	if got.Annotations[NodeGroupUIDAnnotation] != "uid-1" ||
		got.Annotations[StorageClassAnnotation] != remoteTemplate.StorageClass ||
		got.Annotations[DeviceClassAnnotation] != remoteTemplate.DeviceClass {
		t.Fatalf("unexpected annotations: %#v", got.Annotations)
	}
	if got.NodeTopology == nil || got.NodeTopology.MatchLabels[TemplateNodeLabel] != "true" ||
		got.NodeTopology.MatchLabels[NodeGroupNamespaceLabel] != "datadog" ||
		got.NodeTopology.MatchLabels[NodeGroupNameLabel] != "remote-workers" {
		t.Fatalf("unexpected topology: %#v", got.NodeTopology)
	}
	owner := metav1.GetControllerOf(got)
	if owner == nil || owner.Name != "remote-workers" || owner.UID != "uid-1" {
		t.Fatalf("unexpected owner: %#v", owner)
	}

	// A second reconcile of an up-to-date object must not write.
	if err := reconcileOnce(t, r, "remote-workers"); err != nil {
		t.Fatal(err)
	}
	if again := getCapacity(t, c, capacityKey(ng, remoteTemplate)); again.ResourceVersion != got.ResourceVersion {
		t.Fatalf("resourceVersion changed from %s to %s on no-op reconcile", got.ResourceVersion, again.ResourceVersion)
	}
}

func TestReconcileMultipleTemplatesCreateOneCapacityEach(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "uid-1", remoteVolume("data", "lvm", "50Gi"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		ng,
		compatibleStorageClass(remoteTemplate),
		compatibleStorageClass(remoteXFSTemplate),
	).Build()
	r := mustReconciler(t, c, multiConfig)

	if err := reconcileOnce(t, r, "workers"); err != nil {
		t.Fatal(err)
	}
	if got := managedCapacityNames(t, c); len(got) != 2 {
		t.Fatalf("managed capacities = %v, want one per template", got)
	}
	// Each template uses its own spareGB through its own handler invocation.
	for _, tc := range []struct {
		template TemplateConfig
		want     int64
	}{
		{template: remoteTemplate, want: 49*(1<<30) - 8*(1<<20)},
		{template: remoteXFSTemplate, want: 48*(1<<30) - 8*(1<<20)},
	} {
		got := getCapacity(t, c, capacityKey(ng, tc.template))
		if got.StorageClassName != tc.template.StorageClass {
			t.Fatalf("storageClassName = %q, want %q", got.StorageClassName, tc.template.StorageClass)
		}
		if got.Capacity.Value() != tc.want {
			t.Fatalf("%s capacity = %d, want %d", tc.template.StorageClass, got.Capacity.Value(), tc.want)
		}
	}
}

func TestReconcileDispatchesEachTemplateToItsHandler(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "uid-1")
	one := TemplateConfig{StorageClass: "one", DeviceClass: "dc-one", Handler: "handler-one"}
	two := TemplateConfig{StorageClass: "two", DeviceClass: "dc-two", Handler: "handler-two"}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		ng, compatibleStorageClass(one), compatibleStorageClass(two),
	).Build()
	r, err := newReconciler(c, nil, Config{Templates: []TemplateConfig{one, two}}, map[string]CapacityHandler{
		"handler-one": staticCapacityHandler{capacity: quantity("11Gi")},
		"handler-two": staticCapacityHandler{capacity: quantity("22Gi")},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := reconcileOnce(t, r, "workers"); err != nil {
		t.Fatal(err)
	}
	for template, want := range map[TemplateConfig]string{one: "11Gi", two: "22Gi"} {
		got := getCapacity(t, c, capacityKey(ng, template))
		if got.Capacity.Cmp(*quantity(want)) != 0 {
			t.Fatalf("%s capacity = %v, want %s", template.StorageClass, got.Capacity, want)
		}
	}
}

func TestReconcileRepairsManagedDriftAndPreservesMetadata(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "new-uid", remoteVolume("data", "lvm", "20Gi"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ng, compatibleStorageClass(remoteTemplate)).Build()
	r := mustReconciler(t, c, testConfig)

	// Seed an authentic object with an old owner UID, then drift controller-owned
	// fields. A recreated NodeGroup must be allowed to reclaim it safely.
	oldNG := testNodeGroup("workers", "old-uid", remoteVolume("data", "lvm", "10Gi"))
	capacity := r.desiredCapacity(oldNG, remoteTemplate, quantity("1Gi"))
	capacity.Labels["example.com/keep"] = metadataKeepValue
	capacity.Annotations["example.com/keep"] = metadataKeepValue
	capacity.Capacity = resource.NewQuantity(1, resource.BinarySI)
	capacity.MaximumVolumeSize = resource.NewQuantity(2, resource.BinarySI)
	if err := c.Create(context.Background(), capacity); err != nil {
		t.Fatal(err)
	}

	if err := reconcileOnce(t, r, "workers"); err != nil {
		t.Fatal(err)
	}
	got := getCapacity(t, c, client.ObjectKeyFromObject(capacity))
	want := int64(19*(1<<30) - 8*(1<<20))
	if got.Capacity.Value() != want || got.MaximumVolumeSize.Value() != want {
		t.Fatalf("drift not repaired: capacity=%v max=%v", got.Capacity, got.MaximumVolumeSize)
	}
	if got.Labels["example.com/keep"] != metadataKeepValue ||
		got.Annotations["example.com/keep"] != metadataKeepValue {
		t.Fatal("unrelated metadata was not preserved")
	}
	if got.Annotations[NodeGroupUIDAnnotation] != "new-uid" || metav1.GetControllerOf(got).UID != "new-uid" {
		t.Fatalf("stale UID was not repaired: %#v", got.OwnerReferences)
	}
}

func TestReconcileIncompatibleStorageClassDeletesOnlyThatTemplate(t *testing.T) {
	for _, name := range []string{"missing", "wrong-provisioner", "wrong-binding", "wrong-device-class"} {
		t.Run(name, func(t *testing.T) {
			scheme := testScheme(t)
			ng := testNodeGroup("workers", "uid-1", remoteVolume("data", "lvm", "20Gi"))
			// remoteTemplate's StorageClass is broken; remoteXFSTemplate's is
			// healthy and its capacity must be unaffected.
			seed := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ng, compatibleStorageClass(remoteXFSTemplate))
			if name != "missing" {
				sc := compatibleStorageClass(remoteTemplate)
				switch name {
				case "wrong-provisioner":
					sc.Provisioner = "example.com/other"
				case "wrong-binding":
					mode := storagev1.VolumeBindingImmediate
					sc.VolumeBindingMode = &mode
				case "wrong-device-class":
					sc.Parameters[deviceClassParameter] = "other"
				}
				seed.WithObjects(sc)
			}
			c := seed.Build()
			r := mustReconciler(t, c, multiConfig)
			for _, template := range multiConfig.Templates {
				if err := c.Create(context.Background(), r.desiredCapacity(ng, template, quantity("10Gi"))); err != nil {
					t.Fatal(err)
				}
			}

			if err := reconcileOnce(t, r, "workers"); err != nil {
				t.Fatal(err)
			}
			assertCapacityAbsent(t, c, capacityKey(ng, remoteTemplate))
			getCapacity(t, c, capacityKey(ng, remoteXFSTemplate))
		})
	}
}

func TestReconcileStorageClassReadErrorKeepsExistingCapacity(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "uid-1", remoteVolume("data", "lvm", "20Gi"))
	readErr := errors.New("apiserver unavailable")
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ng, compatibleStorageClass(remoteTemplate), compatibleStorageClass(remoteXFSTemplate)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*storagev1.StorageClass); ok && key.Name == remoteTemplate.StorageClass {
					return readErr
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	r := mustReconciler(t, c, multiConfig)
	existing := r.desiredCapacity(ng, remoteTemplate, quantity("10Gi"))
	if err := c.Create(context.Background(), existing); err != nil {
		t.Fatal(err)
	}

	err := reconcileOnce(t, r, "workers")
	if !errors.Is(err, readErr) || !strings.Contains(err.Error(), remoteTemplate.StorageClass) {
		t.Fatalf("Reconcile() error = %v, want wrapped read error for %s", err, remoteTemplate.StorageClass)
	}
	// Unknown compatibility must not delete the existing object.
	if got := getCapacity(t, c, capacityKey(ng, remoteTemplate)); got.Capacity.Cmp(*quantity("10Gi")) != 0 {
		t.Fatalf("existing capacity was modified: %v", got.Capacity)
	}
	// The other template still converges.
	getCapacity(t, c, capacityKey(ng, remoteXFSTemplate))
}

func TestReconcileHandlerWithoutCapacityDeletesOnlyThatTemplate(t *testing.T) {
	for name, noCapacity := range map[string]CapacityHandler{
		"error":        staticCapacityHandler{err: fmt.Errorf("bad storage spec")},
		"nil capacity": staticCapacityHandler{},
	} {
		t.Run(name, func(t *testing.T) {
			scheme := testScheme(t)
			ng := testNodeGroup("workers", "uid-1")
			empty := TemplateConfig{StorageClass: "empty", DeviceClass: "dc", Handler: "empty"}
			healthy := TemplateConfig{StorageClass: "healthy", DeviceClass: "dc", Handler: "healthy"}
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				ng, compatibleStorageClass(empty), compatibleStorageClass(healthy),
			).Build()
			r, err := newReconciler(c, nil, Config{Templates: []TemplateConfig{empty, healthy}}, map[string]CapacityHandler{
				"empty":   noCapacity,
				"healthy": staticCapacityHandler{capacity: quantity("5Gi")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := c.Create(context.Background(), r.desiredCapacity(ng, empty, quantity("1Gi"))); err != nil {
				t.Fatal(err)
			}

			// Invalid NodeGroup storage is not retryable, so no error is returned.
			if err := reconcileOnce(t, r, "workers"); err != nil {
				t.Fatal(err)
			}
			assertCapacityAbsent(t, c, capacityKey(ng, empty))
			getCapacity(t, c, capacityKey(ng, healthy))
		})
	}
}

func TestReconcileCollisionIsUntouchedAndOtherTemplatesConverge(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "uid-1", remoteVolume("data", "lvm", "20Gi"))
	collision := &storagev1.CSIStorageCapacity{ObjectMeta: metav1.ObjectMeta{
		Namespace: "datadog",
		Name:      capacityNameFor(ng, remoteTemplate),
		Labels:    map[string]string{"example.com/owner": "someone-else"},
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		ng, compatibleStorageClass(remoteTemplate), compatibleStorageClass(remoteXFSTemplate), collision,
	).Build()
	r := mustReconciler(t, c, multiConfig)

	if err := reconcileOnce(t, r, "workers"); err == nil {
		t.Fatal("expected collision error")
	}
	got := getCapacity(t, c, client.ObjectKeyFromObject(collision))
	if got.Labels[ManagedByLabel] != "" || got.Labels["example.com/owner"] != "someone-else" {
		t.Fatalf("collision was modified: %#v", got.Labels)
	}
	getCapacity(t, c, capacityKey(ng, remoteXFSTemplate))
}

// TestReconcileDeletesStaleCapacitiesWithoutTouchingOtherObjects seeds a
// capacity from a template that is no longer configured, as left behind by a
// previous process with a different config, alongside objects that must
// survive: an unmanaged look-alike and another NodeGroup's capacity.
func TestReconcileDeletesStaleCapacitiesWithoutTouchingOtherObjects(t *testing.T) {
	scheme := testScheme(t)
	currentNG := testNodeGroup("current", "uid-1", remoteVolume("data", "lvm", "20Gi"))
	otherNG := testNodeGroup("other", "uid-2", remoteVolume("data", "lvm", "20Gi"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		currentNG, otherNG, compatibleStorageClass(remoteTemplate), compatibleStorageClass(remoteXFSTemplate),
	).Build()
	r := mustReconciler(t, c, multiConfig)
	q := quantity("10Gi")

	oldTemplate := remoteTemplate
	oldTemplate.StorageClass = "old-remote-data"
	oldTemplate.DeviceClass = "old-remote-ssd"
	stale := r.desiredCapacity(currentNG, oldTemplate, q)

	unmanagedTemplate := remoteTemplate
	unmanagedTemplate.StorageClass = "unmanaged-remote-data"
	unmanaged := r.desiredCapacity(currentNG, unmanagedTemplate, q)
	delete(unmanaged.Labels, ManagedByLabel)

	other := r.desiredCapacity(otherNG, remoteTemplate, q)
	for _, capacity := range []*storagev1.CSIStorageCapacity{stale, unmanaged, other} {
		if err := c.Create(context.Background(), capacity); err != nil {
			t.Fatal(err)
		}
	}

	if err := reconcileOnce(t, r, "current"); err != nil {
		t.Fatal(err)
	}
	assertCapacityAbsent(t, c, client.ObjectKeyFromObject(stale))
	for _, untouched := range []*storagev1.CSIStorageCapacity{unmanaged, other} {
		getCapacity(t, c, client.ObjectKeyFromObject(untouched))
	}
	for _, template := range multiConfig.Templates {
		getCapacity(t, c, capacityKey(currentNG, template))
	}
}

func TestReconcileInvalidNodeGroupDeletesAllTemplates(t *testing.T) {
	scheme := testScheme(t)
	ng := testNodeGroup("workers", "uid-1", remoteVolume("data", "lvm", "20Gi"))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		ng, compatibleStorageClass(remoteTemplate), compatibleStorageClass(remoteXFSTemplate),
	).Build()
	r := mustReconciler(t, c, multiConfig)
	for _, template := range multiConfig.Templates {
		if err := c.Create(context.Background(), r.desiredCapacity(ng, template, quantity("10Gi"))); err != nil {
			t.Fatal(err)
		}
	}
	// Reconcile a NodeGroup name that is not a valid label value.
	invalid := testNodeGroup(strings.Repeat("a", 64), "uid-1")
	if err := c.Create(context.Background(), invalid); err != nil {
		t.Fatal(err)
	}
	for _, template := range multiConfig.Templates {
		capacity := r.desiredCapacity(invalid, template, quantity("10Gi"))
		if err := c.Create(context.Background(), capacity); err != nil {
			t.Fatal(err)
		}
	}

	if err := reconcileOnce(t, r, invalid.GetName()); err != nil {
		t.Fatal(err)
	}
	for _, template := range multiConfig.Templates {
		assertCapacityAbsent(t, c, capacityKey(invalid, template))
		getCapacity(t, c, capacityKey(ng, template))
	}
}

func TestStorageClassEventsEnqueueAllNodeGroups(t *testing.T) {
	scheme := testScheme(t)
	one := testNodeGroup("one", "uid-1")
	two := testNodeGroup("two", "uid-2")
	two.SetNamespace("other")
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(one, two).Build()
	r := mustReconciler(t, c, multiConfig)

	for _, template := range multiConfig.Templates {
		requests := r.requestsForStorageClass(context.Background(), compatibleStorageClass(template))
		got := map[types.NamespacedName]bool{}
		for _, request := range requests {
			got[request.NamespacedName] = true
		}
		want := []types.NamespacedName{{Namespace: "datadog", Name: "one"}, {Namespace: "other", Name: "two"}}
		if len(requests) != len(want) {
			t.Fatalf("%s: requests = %v, want both NodeGroups", template.StorageClass, requests)
		}
		for _, w := range want {
			if !got[w] {
				t.Fatalf("%s: missing request for %s", template.StorageClass, w)
			}
		}
	}

	unrelated := compatibleStorageClass(remoteTemplate)
	unrelated.Name = "unrelated"
	if got := r.requestsForStorageClass(context.Background(), unrelated); len(got) != 0 {
		t.Fatalf("unrelated StorageClass produced requests: %v", got)
	}
}
