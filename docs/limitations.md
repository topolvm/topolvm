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

TopoLVM removes PVCs on a deleting Node along with Pods using the PVCs.

Normally, StatefulSet controller will re-create Pods and PVCs and schedule
them on other nodes.  However, there can be a small chance that some Pods
are not be rescheduled.

Kubernetes has a validation logic called [Storage Object In Use Protection](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#storage-object-in-use-protection).
This prevents a PVC from removal if it is used by some Pods.

Because of this, TopoLVM needs to remove Pods before PVCs.  Suppose that
the deleting Node is marked as unschedulable by, for example, `kubectl drain`.

The problem happens as follows:

1. TopoLVM removes a Pod.
2. StatefulSet controller re-creates the Pod that still uses a TopoLVM PVC.
3. The Pod becomes pending because it cannot be re-scheduled to the deleting Node.
4. TopoLVM removes the PVC.  Storage Object In Use Protection does not prevent PVC from removal **because the Pod is pending**.
5. StatefulSet does **not** re-craete PVC from the volume claim template.
6. The Pod keeps in pending state because it references non-existent PVC.

TopoLVM can do nothing for this pending Pod because the Pod does not use
TopoLVM PVC now.  We think that it is StatefulSet controller that is
responsible to create a new PVC for the Pod in this case.

Anyway, if this happens, the pending Pod needs to be deleted manually.
