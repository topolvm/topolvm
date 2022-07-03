package controllers

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
)

var testNodeForLogicalVolume = "logicalvolume-test-node"

func testLegacyLogicalVolume(name string) *topolvmlegacyv1.LogicalVolume {
	size := resource.NewQuantity(1<<30, resource.BinarySI)
	return &topolvmlegacyv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Finalizers: []string{
				topolvm.LegacyLogicalVolumeFinalizer,
				"aaa/bbb",
			},
			Labels: map[string]string{
				"aaa": "bbb",
			},
			Annotations: map[string]string{
				"aaa":                              "bbb",
				topolvm.LegacyResizeRequestedAtKey: time.Now().UTC().String(),
			},
		},
		Spec: topolvmlegacyv1.LogicalVolumeSpec{
			Name:        name,
			NodeName:    testNodeForLogicalVolume,
			DeviceClass: topolvm.DefaultDeviceClassName,
			Size:        *size,
		},
		Status: topolvmlegacyv1.LogicalVolumeStatus{
			VolumeID:    "abc",
			Code:        codes.OK,
			Message:     "",
			CurrentSize: size,
		},
	}
}

var _ = Describe("test logicalvolume controller", func() {
	It("should migrate logicalvolumes to new group", func() {
		By("create a legacy logical volume")
		logicalvolume := testLegacyLogicalVolume("test-logicalvolume")
		err := k8sClient.Create(testCtx, logicalvolume)
		Expect(err).ShouldNot(HaveOccurred())

		By("check if legacy logicalvolume resource has been migrated to new group")
		expetedFinalizers := []string{
			// Unrelated finalizers does not remove
			"aaa/bbb",
			// Replaces the legacy TopoLVM finalizer
			topolvm.LogicalVolumeFinalizer,
		}
		sort.Strings(expetedFinalizers)
		expectedAnnotations := map[string]string{
			// Unrelated annotations does not remove
			"aaa": "bbb",
			// Replaces the legacy TopoLVM annotations
			topolvm.ResizeRequestedAtKey: logicalvolume.Annotations[topolvm.LegacyResizeRequestedAtKey],
		}
		expectedLabels := map[string]string{
			"aaa": "bbb",
		}
		expectedSpec := topolvmv1.LogicalVolumeSpec{
			Name:        logicalvolume.Spec.Name,
			NodeName:    logicalvolume.Spec.NodeName,
			Size:        logicalvolume.Spec.Size,
			DeviceClass: logicalvolume.Spec.DeviceClass,
			Source:      logicalvolume.Spec.Source,
			AccessType:  logicalvolume.Spec.AccessType,
		}
		expectedStatus := topolvmv1.LogicalVolumeStatus{
			VolumeID:    logicalvolume.Status.VolumeID,
			Code:        logicalvolume.Status.Code,
			Message:     logicalvolume.Status.Message,
			CurrentSize: logicalvolume.Status.CurrentSize,
		}
		name := types.NamespacedName{
			Name: logicalvolume.Name,
		}

		var success bool
		defer func() {
			if !success {
				err = k8sClient.Delete(testCtx, logicalvolume)
				Expect(err).ShouldNot(HaveOccurred())
			} else {
				newLV := &topolvmv1.LogicalVolume{}
				newLV.Name = logicalvolume.Name
				err = k8sClient.Delete(testCtx, newLV)
				Expect(err).ShouldNot(HaveOccurred())
			}
		}()

		Eventually(func() error {
			lv := &topolvmv1.LogicalVolume{}
			err = k8sClient.Get(testCtx, name, lv)
			if err != nil {
				return fmt.Errorf("can not get the new logicalvolume: err=%v", err)
			}
			sort.Strings(lv.Finalizers)
			if diff := cmp.Diff(expetedFinalizers, lv.Finalizers); diff != "" {
				return fmt.Errorf("the new logicalvolume finalizers does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedAnnotations, lv.Annotations); diff != "" {
				return fmt.Errorf("the new logicalvolume annotations does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedLabels, lv.Labels); diff != "" {
				return fmt.Errorf("the new logicalvolume labels does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedSpec, lv.Spec); diff != "" {
				return fmt.Errorf("the new logicalvolume spec does not match: (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(expectedStatus, lv.Status); diff != "" {
				return fmt.Errorf("the new logicalvolume status does not match: (-want,+got):\n%s", diff)
			}

			legacylv := &topolvmlegacyv1.LogicalVolume{}
			err := k8sClient.Get(testCtx, name, legacylv)
			if err == nil {
				// At first, the legacy logicalvolume finalziers is set, so the legacy logicalvolume is not deleted.
				// So remove the legacy logicalvolume finalizers and see if the legacy logicalvolume is removed in the next loop and beyond.
				// Since this testcase also tests whether the finalizers has been successfully ported to the new logicalvolume, the finalizers cannot be deleted in advance.
				legacylv.Finalizers = []string{}
				err := k8sClient.Update(testCtx, legacylv)
				if err != nil {
					return fmt.Errorf("failed to remove the legacy logicalvolume finalizers: err=%v", err)
				}
				return errors.New("the legacy logicalvolume still exists")
			} else {
				if !apierrs.IsNotFound(err) {
					return fmt.Errorf("failed to get legacy logical volume: err=%v", err)
				}
				// OK if not found the legacy logicalvolume because the legacy logicalvolume is deleted if success migration.
			}
			return nil
		}).Should(Succeed())
		success = true
	})

	It("should not migrate logicalvolumes if NodeName is not match", func() {
		By("create a legacy logical volume")
		logicalvolume := testLegacyLogicalVolume("test-logicalvolume2")
		logicalvolume.Spec.NodeName = "logicalvolume-test-not-matched-node"
		err := k8sClient.Create(testCtx, logicalvolume)
		Expect(err).ShouldNot(HaveOccurred())
		defer func() {
			err = k8sClient.Delete(testCtx, logicalvolume)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		By("check the migration does not process")
		name := types.NamespacedName{
			Name: logicalvolume.Name,
		}
		Consistently(func() error {
			lv := &topolvmv1.LogicalVolume{}
			err = k8sClient.Get(testCtx, name, lv)
			if err == nil {
				return fmt.Errorf("the new logicalvolume is created: lv=%v", lv)
			} else {
				if !apierrs.IsNotFound(err) {
					return fmt.Errorf("failed to get new logical volume: err=%v", err)
				}
				// OK if not found the new logicalvolume because not existing the new logicalvolume indicates that the migration has not been performed.
			}
			legacylv := &topolvmlegacyv1.LogicalVolume{}
			err := k8sClient.Get(testCtx, name, legacylv)
			if err != nil {
				return fmt.Errorf("failed to get legacy logical volume: err=%v", err)
			}
			return nil
		}).WithTimeout(time.Duration(10 * time.Second)).Should(Succeed())
	})
})
