package runners

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testGracePeriod = time.Hour
	uidA            = "11111111-1111-1111-1111-111111111111"
	uidB            = "22222222-2222-2222-2222-222222222222"
	uidC            = "33333333-3333-3333-3333-333333333333"
	uidD            = "44444444-4444-4444-4444-444444444444"
	uidE            = "55555555-5555-5555-5555-555555555555"
	uidF            = "66666666-6666-6666-6666-666666666666"
	uidG            = "77777777-7777-7777-7777-777777777777"
)

func testLV(name, volumeID string, age time.Duration, now time.Time) *topolvmv1.LogicalVolume {
	return &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(now.Add(-age)),
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:     name,
			NodeName: "node1",
		},
		Status: topolvmv1.LogicalVolumeStatus{VolumeID: volumeID},
	}
}

func TestOrphanedVolumeCleaner(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	old := 2 * testGracePeriod

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := topolvmv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	orphan := testLV("pvc-"+uidA, "vol-a", old, now)
	young := testLV("pvc-"+uidB, "vol-b", testGracePeriod/2, now)
	withPV := testLV("pvc-"+uidC, "vol-c", old, now)
	withPVC := testLV("pvc-"+uidD, "vol-d", old, now)
	snapshot := testLV("snapshot-"+uidE, "vol-e", old, now)
	snapshot.Spec.Source = "pvc-" + uidC
	notProvisioned := testLV("pvc-"+uidF, "", old, now)
	unknownName := testLV("custom-volume-name", "vol-g", old, now)
	deleting := testLV("pvc-"+uidG, "vol-h", old, now)
	deleting.Finalizers = []string{topolvm.GetLogicalVolumeFinalizer()}
	deleting.DeletionTimestamp = &metav1.Time{Time: now}
	otherDriverPV := testLV("pvc-99999999-9999-9999-9999-999999999999", "vol-i", old, now)

	pvs := []client.Object{
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-c"},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: topolvm.GetPluginName(), VolumeHandle: "vol-c"},
			}},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-other"},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: "other.csi.example.com", VolumeHandle: "vol-i"},
			}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "claim-d", Namespace: "default", UID: types.UID(uidD)},
	}

	objs := append([]client.Object{
		orphan, young, withPV, withPVC, snapshot, notProvisioned, unknownName, deleting, otherDriverPV, pvc,
	}, pvs...)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	cleaner := NewOrphanedVolumeCleaner(c, c, testGracePeriod).(*orphanedVolumeCleaner)
	cleaner.now = func() time.Time { return now }
	if err := cleaner.cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	lvs := new(topolvmv1.LogicalVolumeList)
	if err := c.List(context.Background(), lvs); err != nil {
		t.Fatal(err)
	}
	remaining := make([]string, 0, len(lvs.Items))
	for _, lv := range lvs.Items {
		remaining = append(remaining, lv.Name)
	}
	sort.Strings(remaining)

	want := []string{
		withPV.Name, withPVC.Name, young.Name, notProvisioned.Name, deleting.Name, "custom-volume-name", snapshot.Name,
	}
	sort.Strings(want)
	if len(remaining) != len(want) {
		t.Fatalf("remaining LogicalVolumes %v, want %v", remaining, want)
	}
	for i := range want {
		if remaining[i] != want[i] {
			t.Fatalf("remaining LogicalVolumes %v, want %v", remaining, want)
		}
	}
}
