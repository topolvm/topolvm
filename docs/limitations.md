Limitations
===========

StorageClass reclaim policy
---------------------------

TopoLVM does not care about `Retain` policy because CSI volumes can be referenced
only via PersistentVolumeClaims.

https://kubernetes.io/docs/concepts/storage/volumes/#csi
> The `csi` volume type does not support direct reference from Pod and may only be referenced in a Pod via a `PersistentVolumeClaim` object.

TopoLVM supports only `reclaimPolicy: Delete` on `StorageClass`.

`StatefulSet`s `Pod` might be `Pending` after rescheduling
----------------------------------------------------------

TopoLVM will cleanup PVCs on the deleting Nodes and Pods using the PVCs.
To avoid [Storage Object In Use Protection](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#storage-object-in-use-protection), Pods need to be deleted before PVCs.

At worst cases, StatefulSet controller may create a new Pod just after
`topolvm-controller` deletes the Pod but before it deletes the PVC.
In this case, the PVC will anyways be deleted by `topolvm-controller`,
and the new Pod will be in the pending state forever because StatefulSet controller
in this case does not instantiate a new PVC from the template.

To solve this problem, user needs to delete the pending `Pod` manually.
