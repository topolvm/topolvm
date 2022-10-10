package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(topolvmv1.AddToScheme(scheme))
	utilruntime.Must(topolvmlegacyv1.AddToScheme(scheme))
}

func main() {
	log.Println("Start migration for example")
	config := ctrl.GetConfigOrDie()
	mapper, err := apiutil.NewDynamicRESTMapper(config)
	if err != nil {
		log.Fatalf("unable to setup mapper: %+v\n", err)
		return
	}
	clientOptions := client.Options{Scheme: scheme, Mapper: mapper}
	c, err := client.New(ctrl.GetConfigOrDie(), clientOptions)
	if err != nil {
		log.Fatalf("unable to setup client: %+v\n", err)
		return
	}

	log.Println("Start convert LogicalVolumes")
	ctx := context.Background()
	lvList := topolvmlegacyv1.LogicalVolumeList{}
	err = c.List(ctx, &lvList)
	if err != nil {
		log.Fatalf("unable to get legacy LogicalVolumeList: %+v\n", err)
		return
	}
	for _, lv := range lvList.Items {
		log.Printf("Process LogicalVolume: %s\n", lv.Name)
		newLV := topolvmv1.LogicalVolume{}
		u := &unstructured.Unstructured{}
		if err := scheme.Convert(&lv, u, nil); err != nil {
			log.Fatalf("unable to convert legacy LogicalVolume to unstractured: %+v\n", err)
			return
		}
		u.SetGroupVersionKind(topolvmv1.GroupVersion.WithKind("LogicalVolume"))
		if err := scheme.Convert(u, &newLV, nil); err != nil {
			log.Fatalf("unable to convert unstractured to LogicalVolume: %+v\n", err)
			return
		}
		if v, ok := newLV.Annotations["topolvm.cybozu.com/resize-requested-at"]; ok {
			newLV.Annotations["topolvm.io/resize-requested-at"] = v
			delete(newLV.Annotations, "topolvm.cybozu.com/resize-requested-at")
		}
		for i := range newLV.Finalizers {
			if newLV.Finalizers[i] == "topolvm.cybozu.com/logicalvolume" {
				newLV.Finalizers[i] = "topolvm.io/logicalvolume"
			}
		}

		newLV.ResourceVersion = ""
		newLV.UID = types.UID("")
		newLV.ManagedFields = []metav1.ManagedFieldsEntry{}
		delete(newLV.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		status := newLV.DeepCopy().Status
		err = c.Create(ctx, &newLV)
		if err != nil {
			log.Fatalf("unable to create LogicalVolume: %+v\nnewLV: %+v\n", err, newLV)
			return
		}
		log.Printf("Created LogicalVolume: %s\n", newLV.Name)

		newLV2 := topolvmv1.LogicalVolume{}
		err = c.Get(ctx, types.NamespacedName{Name: newLV.Name}, &newLV2)
		if err != nil {
			log.Fatalf("unable to get LogicalVolume: %+v\n", err)
			return
		}
		newLV2.Status = status
		err = c.Status().Update(ctx, &newLV2)
		if err != nil {
			log.Fatalf("unable to update LogicalVolume status: %+v\nnewLV: %+v\n", err, newLV2)
			return
		}
		log.Printf("Updated LogicalVolume: %s\n", newLV2.Name)
	}

	log.Println("Start update PersistentVolumeClaims")
	pvcList := corev1.PersistentVolumeClaimList{}
	err = c.List(ctx, &pvcList)
	if err != nil {
		log.Fatalf("unable to get PersistentVolumeClaimList: %+v\n", err)
		return
	}
	for _, pvc := range pvcList.Items {
		if pvc.Annotations["volume.beta.kubernetes.io/storage-provisioner"] == "topolvm.cybozu.com" {
			pvc2 := pvc.DeepCopy()
			pvc2.Name += "-new"
			log.Printf("Process PersistentVolumeClaim: %s\n", pvc2.Name)
			delete(pvc2.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			pvc2.Annotations["volume.beta.kubernetes.io/storage-provisioner"] = "topolvm.io"
			pvc2.Annotations["volume.kubernetes.io/storage-provisioner"] = "topolvm.io"
			pvc2.Spec.VolumeName += "-new"
			scName := fmt.Sprintf("%s-new", *pvc2.Spec.StorageClassName)
			pvc2.Spec.StorageClassName = &scName
			pvc2.ResourceVersion = ""
			pvc2.UID = types.UID("")
			err = c.Create(ctx, pvc2)
			if err != nil {
				log.Fatalf("unable to create PersistentVolumeClaim: %+v\npvc: %+v\n", err, pvc2)
				return
			}
			log.Printf("Created PersistentVolumeClaim: %s\n", pvc2.Name)
		}
	}

	log.Println("Start update PersistentVolumes")
	pvList := corev1.PersistentVolumeList{}
	err = c.List(ctx, &pvList)
	if err != nil {
		log.Fatalf("unable to get PersistentVolumeList: %+v\n", err)
		return
	}
	for _, pv := range pvList.Items {
		if pv.Annotations["pv.kubernetes.io/provisioned-by"] == "topolvm.cybozu.com" {
			pv2 := pv.DeepCopy()
			pv3 := pv.DeepCopy()
			log.Printf("Process PersistentVolume(): %s\n", pv3.Name)

			pv2.Spec.PersistentVolumeReclaimPolicy = "Retain"
			err = c.Update(ctx, pv2)
			if err != nil {
				log.Fatalf("unable to update PersistentVolume: %+v\npv: %+v\n", err, pv2)
				return
			}
			err = c.Delete(ctx, pv2)
			if err != nil {
				log.Fatalf("unable to delete legacy PersistentVolume %q: %+v\n", pv2.Name, err)
				return
			}

			pv3.Name += "-new"
			delete(pv3.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			pv3.Annotations["pv.kubernetes.io/provisioned-by"] = "topolvm.io"
			pv3.Spec.CSI.Driver = "topolvm.io"
			pv3.Spec.StorageClassName += "-new"
			pv3.Spec.CSI.VolumeAttributes["storage.kubernetes.io/csiProvisionerIdentity"] = strings.Replace(pv3.Spec.CSI.VolumeAttributes["storage.kubernetes.io/csiProvisionerIdentity"], "topolvm.cybozu.com", "topolvm.io", -1)
			pv3.ResourceVersion = ""
			pv3.UID = types.UID("")
			pv3.ManagedFields = []metav1.ManagedFieldsEntry{}
			if pv3.Spec.NodeAffinity != nil && pv3.Spec.NodeAffinity.Required != nil {
				for i := range pv3.Spec.NodeAffinity.Required.NodeSelectorTerms {
					for l := range pv3.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions {
						if pv3.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[l].Key == "topology.topolvm.cybozu.com/node" {
							pv3.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[l].Key = "topology.topolvm.io/node"
						}
					}
				}
			}

			pv3.Spec.ClaimRef.Name += "-new"
			pvc := corev1.PersistentVolumeClaim{}
			err = c.Get(ctx, types.NamespacedName{Name: pv3.Spec.ClaimRef.Name, Namespace: pv3.Spec.ClaimRef.Namespace}, &pvc)
			if err != nil {
				log.Fatalf("unable to get PersistentVolumeClaim: %+v\n", err)
				return
			}
			pv3.Spec.ClaimRef.ResourceVersion = pvc.ResourceVersion
			pv3.Spec.ClaimRef.UID = pvc.UID
			err = c.Create(ctx, pv3)
			if err != nil {
				log.Fatalf("unable to create PersistentVolume: %+v\npv: %+v\n", err, pv3)
				return
			}
			log.Printf("Created PersistentVolume: %s\n", pv3.Name)
		}
	}

	log.Println("Complete migration for example")
}
