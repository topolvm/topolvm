---
title: pvcs-smaller-than-1gi
authors:
  - "@jakobmoellerdev"
reviewers:
  - "@pluser"
  - "@toshipp"
  - "@suleymanakbas91"
  - "@jeff-roche"
  - "@brandisher"
approvers:
  - "@pluser"
  - "@toshipp"
api-approvers:
  - "@toshipp"
creation-date: 2023-09-23
last-updated: 2023-09-26
---

# Supporting PVCs smaller than 1 Gi in size

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
- [Proposal](#proposal)
    - [Decision Outcome](#decision-outcome)
    - [Open Questions](#open-questions)
- [Design Details](#design-details)
    - [How to pass byte-level capacities to TopoLVM?](#how-to-pass-byte-level-capacities-to-topolvm)
    - [How to get around the `<<30`/`>>30` bit shifts](#how-to-get-around-the-3030-bit-shifts)
    - [Impact Analysis on Existing PVC creation](#impact-analysis-on-existing-pvc-creation)
    - [Changing how `LogicalVolumeService` is creating `LogicalVolumeSpec`](#changing-how-logicalvolumeservice-is-creating-logicalvolumespec)
    - [Modifying `controllers/logicalvolume_controller` to create correct LVMD calls via gRPC](#modifying-controllerslogicalvolumecontroller-to-create-correct-lvmd-calls-via-grpc)
    - [Switching all occurrences of `uint64` to `int64` in lvmd and change `size_gb` to `size` in protobuf messages.](#switching-all-occurrences-of-uint64-to-int64-in-lvmd-and-change-sizegb-to-size-in-protobuf-messages)
    - [Dealing with less than 1Gi storage in the TopoLVM scheduler prioritization](#dealing-with-less-than-1gi-storage-in-the-topolvm-scheduler-prioritization)
- [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
- [Deprecation / Removal](#deprecation--removal)
<!-- /toc -->

## Summary

Currently, TopoLVM lives with a design decision to have the smallest amount of storage be limited to 1 GB.
This is mainly due to various bit shifting and assumptions made in the code and not because of limitations in CSI, Kubernetes or LVM.
We need to remove this sizing limitation.

## Motivation

For most users of TopoLVM, local nodes are the focus. 
While TopoLVM was designed to be run in datacenters where the lowest provisional unit might not be an issue, edge use cases require us to create many volumes in a small amount of storage which may only be a few gigabytes.
To allow TopoLVM for proper use on edge devices and small form-factor use cases of Pods, we need to allow storage capacities that can supply storage lower than 1GB.

### Goals

- Ensure backwards-compatibility with schedulers and legacy bit shifts that would be replaced
- Discover all places with hardcoded bit shifts and transition them to a more modular and sizable approach
- Allow creation of PVCs that request less than 1GB of storage, with a minimum size that is reasonably set.

### Current Workflow

1. TopoLVM administrator installs TopoLVM as usual
2. Cluster user creates PVCs with a Storage Class backed by TopoLVM following [the process of PVC creation during dynamic provisioning](../design.md#how-dynamic-provisioning-works):
    ```bash
    $ cat <<EOF | kubectl apply -f -
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: test-claim
    spec:
      storageClassName: vg1
      resources:
        requests:
          storage: 500Mi
      accessModes:
        - ReadWriteOnce
      volumeMode: Filesystem
    EOF
    ```
3. Once the PVC is used, the [`convertRequestCapacity` method in `topolvm-controller` and `topolvm-node`](../../driver/controller.go) kicks in and defaults to `1Gi` of storage as request capacity, resulting in an error
   if we have only `500Mi` or less storage available to us. We want to change step 3 to allow capacities below 1Gi of storage.

## Proposal

The main idea of the proposal is to remove all in-code limitations that are currently in place that prohibit us from allowing sizing less than 1Gi to any PVCs.

The main reason of hard coding 1Gi as minimum size is the `<<30`/`>>30` bitshift.
Any occurrence of the bit shift needs to be identified and replaced with a more dynamic version.

In total the entire codebase contains 73 occurrences of `<<30` and 8 occurrences of `>>30` as of writing this proposal.
While it would not be completely trivial to replace, we believe this is still a reasonable amount to refactor with manual care.

This means we need to **find all occurrences of `<<30` Bit shifts and `GB` values and replace them with _byte-level_ comparisons**.

Breaking this down we will tackle the following questions:
1. [How to pass byte-level capacities to TopoLVM?](#how-to-pass-byte-level-capacities-to-topolvm) 
    
    This will allow us to actually get the necessary capacity precision required to support less than 1Gi PVCs.
    We want to achieve this by using the inbuilt [byte-level precision in the CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md#controller-service-rpc).

2. [How to get around the `<<30`/`>>30` bit shifts?](#how-to-get-around-the-3030-bit-shifts) 

    This will allow us to use code paths in TopoLVM that do not require comparisons with Gi-level precision.
    We are showcasing how it is possible to get around the bitshift and how to replace it with byte-level comparisons.

### API Changes

Additionally, as a result of an [Impact Analysis of the code path when creating a PVC](#impact-analysis-on-existing-pvc-creation) we will be facing two breaking API changes:

1. **Any request/response made with lvmd using a `size_gb` field needs to be changed to a different `size` because otherwise we cannot communicate less than 1Gi capacities to `lvmd`.**
2. **The [current prioritisation algorithm within the TopoLVM Scheduler Extension](../topolvm-scheduler.md#prioritize) needs to be adjusted and its current version needs to specify a limitation when working with storage less than 1Gi**

### Code Changes related to the API 

We are proposing how to change the current codebase [considering the breaking changes in the API](#api-changes):
- How to deal with breaking changes in LVMD?
   - [Switching all occurrences of `uint64` to `int64` in lvmd and change `size_gb` to `size` in protobuf messages.](#switching-all-occurrences-of-uint64-to-int64-in-lvmd-and-change-sizegb-to-size-in-protobuf-messages)
- How to touch important components which are using bit shifts that were previously using `size_gb`
    - [Changing how `LogicalVolumeService` is creating `LogicalVolumeSpec`](#changing-how-logicalvolumeservice-is-creating-logicalvolumespec)
    - [Modifying `controllers/logicalvolume_controller` to create correct LVMD calls via gRPC](#modifying-controllerslogicalvolumecontroller-to-create-correct-lvmd-calls-via-grpc)
- How to deal with breaking changes in the TopoLVM Scheduler?
   - [Dealing with less than 1Gi storage in the TopoLVM scheduler prioritization](#dealing-with-less-than-1gi-storage-in-the-topolvm-scheduler-prioritization)

### Decision Outcome


### Open Questions

Under the assumption that the change is deemed useful or accepted, we still need to decide on the following points:

1. Should `lvmd` protobuf message set be considered `user-facing`? If so should we `reserve` or `deprecate` during the change? If not, we can remove the field. For more details see [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
2. Should we duplicate test cases for less than 1Gi volume sources, or adjust existing tests to cover the new scenarios?
3. Should the old scheduling algorithm be kept with a new version to choose from and noted as being limited in the old version for storage less than 1Gi or should it be replaced/removed?

## Design Details


### How to pass byte-level capacities to TopoLVM?

The answer to this question lies in `topolvm-controller` and `topolvm-node` when called by Kubernetes.

With every call made to a volume request (e.g. `CreateVolume`), a `CapacityRange` is included:

```go
// The capacity of the storage space in bytes. To specify an exact size,
// `required_bytes` and `limit_bytes` SHALL be set to the same value. At
// least one of the these fields MUST be specified.
type CapacityRange struct {
	// Volume MUST be at least this big. This field is OPTIONAL.
	// A value of 0 is equal to an unspecified field value.
	// The value of this field MUST NOT be negative.
	RequiredBytes int64 `protobuf:"varint,1,opt,name=required_bytes,json=requiredBytes,proto3" json:"required_bytes,omitempty"`
	// Volume MUST not be bigger than this. This field is OPTIONAL.
	// A value of 0 is equal to an unspecified field value.
	// The value of this field MUST NOT be negative.
	LimitBytes           int64    `protobuf:"varint,2,opt,name=limit_bytes,json=limitBytes,proto3" json:"limit_bytes,omitempty"`
	// ...
}
```

As a result, all capacity ranges in CSI are coming from `req.GetCapacityRange().GetRequiredBytes()` and `req.GetCapacityRange().GetLimitBytes()`.

**Important here: both are raw byte counts in the form of `int64`.**


_However_, We currently convert all request capacity with the `convertRequestCapacity` method

```go
func convertRequestCapacity(requestBytes, limitBytes int64) (int64, error) {
	if requestBytes < 0 {
		return 0, errors.New("required capacity must not be negative")
	}
	if limitBytes < 0 {
		return 0, errors.New("capacity limit must not be negative")
	}

	if limitBytes != 0 && requestBytes > limitBytes {
		return 0, fmt.Errorf(
			"requested capacity exceeds limit capacity: request=%d limit=%d", requestBytes, limitBytes,
		)
	}

	if requestBytes == 0 {
		return 1, nil
	}
	return (requestBytes-1)>>30 + 1, nil
}
```

How do we get around this?

**By replacing this method with a method that simply checks for limitBytes and then returning the proper requestBytes, we can easily
push that value into all other parts of the code, making it available to every part of the stack called by either `topolvm-controller` or `topolvm-node`.**

### How to get around the `<<30`/`>>30` bit shifts

As one can see below we can simply get around the bitshift by using the full byte definition instead of using a scaled version of `gb`:

```go
package main

import (
	"fmt"

	. "k8s.io/apimachinery/pkg/api/resource"
)

func main() {
	var oldDefGi = int64(1)
	var newDefGi = MustParse(fmt.Sprintf("%vGi", oldDefGi))
	var someGiBytes = int64(1073741824)

	var oldDefMi = int64(500)
	var newDefMi = MustParse(fmt.Sprintf("%vMi", oldDefMi))
	var someMiBytes = int64(524288000)

	println(someGiBytes == oldDefGi<<30) // always true
	println(NewQuantity(someGiBytes, BinarySI).Cmp(newDefGi) == 0) // always true

	println(someMiBytes == oldDefMi<<30) // always false
	println(NewQuantity(someMiBytes, BinarySI).Cmp(newDefMi) == 0) // always true
}

```

At the same time we already make use of the serialized Capacity in `LogicalVolume` for parsing into JSON or YAML so we do not have to break user-facing Kubernetes APIs.
Since all CSI Driver values already work with bytes, we have no trouble taking in the new data, we will just accept more ranges.

We will make 2 changes to LVMD:
1. We will accept a breaking change in `lvmd` that moves from `size_gb` to a more flexible `size` when relating to request / response sizes.
2. We will move from `uint64` comparisons in `lvmd` to `int64` comparisons. This is the same level of precision as within CSI driver specifications.

Together with changes in LVMD we can easily replace all bit shifted comparisons with byte level comparisons based on `int64`.

Within tests, we will write a small helper function that easily allows defining `int64` for any amount of Gi that we previously used bitshifts for.


### Impact Analysis on Existing PVC creation

When following the [architecture of a PVC creation during dynamic provisioning](../design.md#how-dynamic-provisioning-works), we can observe the following steps:

1. >`external-provisioner` finds a new unbound PersistentVolumeClaim (PVC) for TopoLVM.

   For this step, we can keep the existing call in place and do not need to touch anything.

2. >`external-provisioner` calls CSI controller's `CreateVolume` with the topology key of the target node.

   For this step, we will need to adjust how `CreateVolume` passes the requested storage to allow less than 1Gi being a valid capacity.

3. >`topolvm-controller` creates a `LogicalVolume` with the topology key and capacity of the volume.

   For this step, we will need to make sure that `topolvm-controller` is able to handle less than 1Gi storage as a valid capacity inside `LogicalVolume`.

4. > `topolvm-node` on the target node finds the `LogicalVolume`.

   We do not need to touch the existing discovery logic.

5. > `topolvm-node` sends a volume create request to `lvmd`.

   **BREAKING**: We need to make sure that the request sent to lvmd via gRPC is able to convey capacities less than 1Gi, which is currently not possible.
   Similarly, all responses must be able to work with less than 1Gi capacities. 
   More details on this can be found in the section on the [Impact on LVMD protocol changes](#switching-all-occurrences-of-uint64-to-int64-in-lvmd-and-change-sizegb-to-size-in-protobuf-messages).

6. > `lvmd` creates an LVM logical volume as requested.

   No changes to creation are expected, since lvm2 can deal with less than 1Gi storage already.

7. > `topolvm-node` updates the status of `LogicalVolume`.

   No changes are expected as the `LogicalVolume` status supports reporting capacities less than 1Gi

8. > `topolvm-controller` finds the updated status of `LogicalVolume`.

   As long as `LogicalVolume` has the correct capacities we do not need to change anything here.

9. > `topolvm-controller` sends the success (or failure) to `external-provisioner`.

   No changes are expected after sizing has been completed

10. > `external-provisioner` creates a PersistentVolume (PV) and binds it to the PVC.

    No changes are expected after sizing has been completed


Additionally, any TopoLVM user making use of the [scheduler extension](../topolvm-scheduler.md) will also be impacted by the following changes:

1. > `topolvm-node` exposes free storage capacity as `capacity.topolvm.io/<device-class>` annotation of each Node.

   These capacities are already reported on byte-level, not in Gi, so they do not need to be touched
2. > `topolvm-controller` works as a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for new Pods.
   > - It adds `capacity.topolvm.io/<device-class>` annotation to a pod and `topolvm.io/capacity` resource to the first container of a pod.
   > - The value of the annotation is the sum of the storage capacity requests of unbound TopoLVM PVCs for each volume group referenced by the pod.

   This logic currently rounds up to the next GB value and needs to be modified from `requested = ((req.Value()-1)>>30 + 1) << 30` to `requested = req.Value()`
   for the [Mutating Webhook](https://github.com/topolvm/topolvm/blob/main/hook/mutate_pod.go#L248) in all places.
   Otherwise, the capacity annotation will be incorrect.
    ```go
    requested int64 := topolvm.DefaultSize
    if req, ok := volumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
      if req.Value() != 0 {
        requested = req.Value()
      }
    }
    ```

3. > `topolvm-scheduler` filters and scores Nodes for a new pod having `topolvm.io/capacity` resource request.
   > - Nodes having less capacity in given volume group than requested are filtered.
   > - Nodes having larger capacity in given volume group are scored higher.

   **BREAKING**: This logic will need to be adjusted since the current scheduling algorithm is based on a divisor that works on Gi increments.
   The only way to resolve this is by changing the scheduling or using the [Kubernetes Storage Capacity feature](https://kubernetes.io/docs/concepts/storage/storage-capacity/) instead of the TopoLVM scheduler extension.
   More details on this can be found in the section on the [Impact on the TopoLVM scheduler extension](#dealing-with-1gi-storage-in-the-topolvm-scheduler-prioritization).

### Changing how `LogicalVolumeService` is creating `LogicalVolumeSpec`

TopoLVM sets bit shifted sizing for all current `LogicalVolumeSpec` objects when calling [`LogicalVolumeService.CreateVolume`](../../driver/internal/k8s/logicalvolume_service.go).
They are always defined with `*resource.NewQuantity(requestGb<<30, resource.BinarySI)`, however this definition is wrong.
Instead, it should have a scaled quantity `*resource.NewQuantity(requestSize, resource.BinarySI)` after which all calls will work fine again.
No Changes to the `LogicalVolume` CRD are necessary.

### Modifying `controllers/logicalvolume_controller` to create correct LVMD calls via gRPC

Since the main TopoLVM controllers already use `reqBytes := lv.Spec.Size.Value()` for their sizing the only thing necessary
is to change the calls of LVMD (protocol modifications below).

Calls will directly include the request / response bytes like so:

```go
    resp, err := r.lvService.CreateLVSnapshot(ctx, &proto.CreateLVSnapshotRequest{
        Name:         string(lv.UID),
        DeviceClass:  lv.Spec.DeviceClass,
        SourceVolume: sourceVolID,
+/-     Size:         reqBytes,
        AccessType:   lv.Spec.AccessType,
    })
```

will have to have `SizeGb` replaced with `Size` and their raw values so we can get rid of the bitshift. This is only
possible by introducing a breaking change to LVMD protocol.

### Switching all occurrences of `uint64` to `int64` in lvmd and change `size_gb` to `size` in protobuf messages.

Since lvmd currently includes various messages with a `uint64 size_gb` field, we should think of how to properly
serialize new the capacity information in a scalable way. Here we have an inbuilt option with the kubernetes quantities as well.

we could simply remove this limitation and pass requestBytes natively as its byte count into LVMD:

```protobuf
    // Represents the input for CreateLV.
    message CreateLVRequest {
      string name = 1;              // The logical volume name.
+/-   uint64 size_gb = 2 [deprecated = true];  // Volume size in GiB. Deprecated in favor of size
      repeated string tags = 3;     // Tags to add to the volume during creation
      string device_class = 4;
      string lvcreate_option_class = 5;
  
+     int64 size = 6; // Volume size in canonical bytes.
    }
```

Note that SizeGB is used as uint64 where we cast down to int64. This means that potentially,
if someone had a volume before this change greater than `9.223.372.036.854.775.807 Gi`, he would now experience an overflow
that would break lvmd. *However*, we need to be aware that the CSIDriver capacity-ranges at most support values up to int64 limits,
so we do not break any currently known path.

This change from `size_gb` to `size` can be reused across all Requests/Responses and only needs minor adjustment.

There is a second part to this change: the json parsing in `lvmd/command`: This is currently the root of why all comparisons need to be done with `uint64` instead of `int64`.

We can replace all occurrences during the parsing process to get arround this:

```go
    type vg struct {
        name string
        uuid string
+/-     size int64
+/-     free int64
    }
    
    func (u *vg) UnmarshalJSON(data []byte) error {
        type vgInternal struct {
            Name string `json:"vg_name"`
            UUID string `json:"vg_uuid"`
            Size string `json:"vg_size"`
            Free string `json:"vg_free"`
        }
    
        var temp vgInternal
        if err := json.Unmarshal(data, &temp); err != nil {
            return err
        }
    
        u.name = temp.Name
        u.uuid = temp.UUID
    
        var convErr error
+/-     u.size, convErr = strconv.ParseInt(temp.Size, 10, 64)
        if convErr != nil {
            return convErr
        }
+/-     u.free, convErr = strconv.ParseInt(temp.Free, 10, 64)
        if convErr != nil {
            return convErr
        }
    
        return nil
    }
```

Of course the same adjustments from `uint64` to `int64` be done for all methods using these structs and similar structs like `lv`.  
Then they can be reused in `lvmd/lvservice` and `lvmd/vgservice` to simplify the calls from uint64 bitshift comparisons to simple byte comparisons:
The resulting `CreateLV` call for example would look almost exactly like the original `CreateLV`:

```go
    func (s *lvService) CreateLV(_ context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
        dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
        // ...
        vg, err := command.FindVolumeGroup(dc.VolumeGroup)
        // ...
        oc := s.ocmapper.LvcreateOptionClass(req.LvcreateOptionClass)
+/-     requested := req.GetSize()
+/-     free := int64(0)
        var pool *command.ThinPool
        switch dc.Type {
        case TypeThick:
            free, err = vg.Free()
            // ...
        case TypeThin:
            pool, err = vg.FindPool(dc.ThinPoolConfig.Name)
            // ...
            tpu, err := pool.Free()
            // ...
+/-         free = int64(math.Floor(dc.ThinPoolConfig.OverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
        default:
            // technically this block will not be hit however make sure we return error
            // in such cases where deviceclass target is neither thick or thinpool
            return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
        }

+/-     if free < requested {
            log.Error("no enough space left on VG", map[string]interface{}{
                "free":      free,
                "requested": requested,
            })
            return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
        }
        // ...
        var lv *command.LogicalVolume
        switch dc.Type {
        case TypeThick:
            lv, err = vg.CreateVolume(req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
        case TypeThin:
            lv, err = pool.CreateVolume(req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
        default:
            return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
        }
        // ...
        return &proto.CreateLVResponse{
            Volume: &proto.LogicalVolume{
                Name:     lv.Name(),
+/-             Size:     lv.Size(),
                DevMajor: lv.MajorNumber(),
                DevMinor: lv.MinorNumber(),
            },
        }, nil
    }
```

### Dealing with less than 1Gi storage in the TopoLVM scheduler prioritization

Currently, the scheduler uses the following scoring methodology for a node: `min(10, max(0, log2(capacity >> 30 / divisor)))`
Note that the actual value of the capacity is stored in an annotation of the form `topolvm.GetCapacityKeyPrefix()+dc`,
and _more importantly_, contains the value of the capacity in bytes.

Our simplest suggestion here, in order to not break anything, is to clearly mark the existing capacity calculation as inaccurate and introduce
a new lower bound for less than 1Gi storage in the method `capacityToScore`:

```go
// DEPRECATED: capacityToScore uses gi precision and a divisor as per formula 
//  min(10, max(0, log2(capacity >> 30 / divisor))) for (capacity>>30)>0
// Its limited to score capacities 1Gi>capacity>0 with 1 without any detailed precision.
// Note that this is a legacy scheduler and should not be used for volumes smaller 1Gi due to this limitation.
func capacityToScore(capacity uint64, divisor float64) int {
+/-	gb := capacity >> 30
+/-
+/-	// we have a capacity that is greater than 0 but less than 1Gi, apply score 1
+/-	if capacity > 0 && gb == 0 {
+/-		return 1
+/-	}
	// avoid logarithm of zero, which diverges to negative infinity.
	if gb == 0 {
		return 0
	}

	converted := int(math.Log2(float64(gb) / divisor))
	switch {
	case converted < 0:
		return 0
	case converted > 10:
		return 10
	default:
		return converted
	}
}
```

This leads to every free Capacity between 1 byte and 1GB free to get the score of `1`, which means they will be scored the same.
The only downside to this algorithm is that its precision is 1GB. So if there are 2 nodes with `500Mi` and `700Mi` free storage, both would receive the same score.

The only way to get around this is by creating a new scoring algorithm side-by-side and allow users to switch. 
However, this proposal will only focus on solving the existing scheduling mechanism and will thus recommend to simply make the old scheduler work.
A separate proposal should be created for the new scheduling algorithm.

For most users, this should be a sufficient alternative while providing high enough accuracy for the majority of cases.

### Upgrade / Downgrade Strategy

An upgrade will be seamless and not cause any issues with inflight messages into lvmd.

A downgrade will work seamless as well with any component no matter the restarts or order of downgrade, since we have a nil check on size quantity and will fallback
to legacy size calculations instead of the new size_quantity.

### Deprecation / Removal

First we can easily deprecate `size_gb` from any messages that contain it:

```protobuf
    // Represents the input for CreateLV.
    message CreateLVRequest {
      string name = 1;              // The logical volume name.
+/-   uint64 size_gb = 2 [deprecated = true];
      repeated string tags = 3;     // Tags to add to the volume during creation
      string device_class = 4;
      string lvcreate_option_class = 5;
+     int64 size = 6; // Volume size in bytes
    }
```

We can then easily remove the legacy `size_gb` field in a future release by making use of `reserved` if we are sure there will be no future usages in the next release:

```protobuf
    // Represents the input for CreateLV.
    message CreateLVRequest {
      string name = 1;              // The logical volume name.
+/-   reserved 2;
      repeated string tags = 3;     // Tags to add to the volume during creation
      string device_class = 4;
      string lvcreate_option_class = 5;
+     int64 size = 6; // Volume size in bytes
    }
```