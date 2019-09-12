Limitations
===========

StorageClass reclaim policy
---------------------------

TopoLVM does not care about `Retain` [reclaim policy](https://kubernetes.io/docs/concepts/storage/storage-classes/#reclaim-policy)
because CSI volumes can be referenced only via PersistentVolumeClaims.

Ref: https://kubernetes.io/docs/concepts/storage/volumes/#csi

> The `csi` volume type does not support direct reference from Pod and may
> only be referenced in a Pod via a `PersistentVolumeClaim` object. 

StatefulSet Pods may become pending after node removal
------------------------------------------------------

TopoLVM removes PVCs and Pods using the PVCs when a node is being deleted.

This should reschedule Pods and PVCs using TopoLVM onto other nodes,
but there can be a small chance that some pods may become pending state,
i.e. not rescheduled to other nodes.

This can happen because, for [Storage Object In Use Protection](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#storage-object-in-use-protection),
TopoLVM needs to delete Pods before PVCs.

In an unfortunate case, StatefulSet controller can re-create the deleted
pod before TopoLVM removes the PVC.  If this happens, TopoLVM then anyway
removes the PVC and **StatefulSet controller will not re-create the PVC
from the volume claim template**.

The re-created pod will be left without its PVC(s) defined and become pending.
TopoLVM can do nothing for this pod because now the pod has nothing related
to TopoLVM.

The pending pod needs to be removed manually to allow StatefulSet to re-create
new Pod and PVC.
