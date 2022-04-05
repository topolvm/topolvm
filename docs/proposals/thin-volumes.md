# Topolvm: Thin Provisioning

## Summary
Thin provisioning will allow topolvm users to create thin logical volumes using thin-pools.

## Motivation
- Storage space can be used more effectively. More users can be accommodated for the same amount of storage space when compared to thick provisioning. This significantly reduces upfront hardware cost for the storage admins.

- Faster clones and snapshots.

### Goals
- As a storage admin, I want to dynamically provision disk space to users on demand based on how much space they need at a given time, so that I can manage unused disk space efficiently.

- As a storage user,  I want to be able to create a large PV, so that I don’t have to worry about extending the PV over and over again.

- As a storage admin, I want to be alerted if the available size of Volume Group with thin logical volumes crosses a certian threshold, so that I can take precautionary measures like extending the volume group.

## Proposal
- Topolvm is **not** responsible for creating the thin-pools. Storage admin/operator has to create the thin-pools in advance while creating the volume groups.
- Multiple device classes can be mapped to a single volume group having multiple thin pools. But each device class should map to a single thin pool within that volume group.
- `lvmd.yaml` file should be updated to hold the mapping between device classes, volume group and thin pools.
- For device classes mapped to thin pools, the node annotation `capacity.topolvm.cybozu.com/<device-class>` should be updated with respect to the thin pool size and overprovision ratio.
- Metrics related to thin volume `data` and `metadata` parameters should be exported.



## Design Details
- A storage admin has to create the volume groups and the thin-pools on this volume group.
- The storage admin should update the lvmd.yaml config file on the nodes to map device classes, volume groups and the corresponding thin-pools.
- Sample `lvmd.yaml` config file:
``` yaml
device-classes:
  - name: ssd-thin
    volume-group: myvg1
    spare-gb: 10
    type: thin
    thin-pool:
        name: pool0
        overprovision-ratio: 50.0
  - name: ssd
    volume-group: myvg1
    default: true
    spare-gb: 10
```
- Above example shows two device classes; `ssd-thin` and `ssd`. Both use `myvg1` volume group.
- `ssd-thin` has thin pool parameters.
- Only thin logical volumes will be created when `ssd-thin` device class is selected by the user in the storageClass parameter `topolvm.cybozu.com/device-class`
- Only thick logical volumes will be created when user has specified `ssd` in the storageClass.

#### API Changes

- The `DeviceClass` struct of the device-class-manager will hold the information about the thin-pool associated with the Volume Group.

``` go
+ type DeviceType string

+ const (
+   TypeThin  = DeviceType("thin")
+   TypeThick = DeviceType("thick")
+ )

+ type ThinPoolConfig struct {
+   // Name of thin pool
+   Name string`json:"name"`
+   // OverProvisionRatio represents the ratio of overprovision that can be allowed on thin pools
+   // and allowed values are greater than 1.0
+   OverProvisionRatio float64 `json:"overprovision-ratio"`
+ }

type DeviceClass struct {
    // Name for the device-class name
    Name string `json:"name"`
    // Volume group name for the device-class
    VolumeGroup string `json:"volume-group"`
    // Default is a flag to indicate whether the device-class is the default
    Default bool `json:"default"`
    // SpareGB is storage capacity in GiB to be spared
    SpareGB *uint64 `json:"spare-gb"`
    // Stripe is the number of stripes in the logical volume
    Stripe *uint `json:"stripe"`
    // StripeSize is the amount of data that is written to one device before moving to the next device
    StripeSize string `json:"stripe-size"`
    // LVCreateOptions are extra arguments to pass to lvcreate
    LVCreateOptions []string `json:"lvcreate-options"`
+   // Type of the volume that this device-class is targetting, TypeThick (default) or TypeThin
+   Type DeviceType `json:"type"`
+   // ThinPoolConfig represents the configuration options for creating thin volumes
+   ThinPoolConfig *ThinPoolConfig `json:"thin-pool"`
}
```
- Following new fields will be added to the API:
    - **Type**: Represents the type of the volume provisioning. Possible values are `thin` and `thick`. This field is optional. If no type is provided, then thick volumes should be provisioned.
    - **ThinPoolConfig**: Contains the configuration of thin pool used for creating thin volumes. This field must exist if the `type` field is `thin` and ignored if the `type` field is not provided or the value is set to `thick`.
        - **Name**: Name of the thin pool
        - **OverProvisionRatio**: This represents the amount of overprovisioning allowed for the thin pool on the node. For example: If the overprovisioningRatio is `5.0`, pool size is 100GiB, then this pool can be overprovisioned upto (5*100) 500GiB. This means that the sum of the virtual sizes of the thin volumes provisioned on the pool per node cannot cross 500GiB. Any value above 1.0 is allowed for this field.


#### Selecting Thin-pool
- The current behavior uses `topolvm.cybozu.com/device-class` parameter on the StorageClass to decide which volume group will be used.
- If `DeviceClass.Type` is `thin`, then thin volumes should be created using `DeviceClass.ThinPoolConfig.Name` thin-pool.
- If `DeviceClass.Type` is `thick` or empty, then thick volumes should be created.


#### LVMD service
- LVService reads the DeviceClass config in the lvmd.yaml.
- If `DeviceClass.Type` is `thin`, `CreateLVRequest` should create thin volumes using `DeviceClass.ThinPoolConfig.Name` thin-pool.
- When creating thin logical volumes, the service should not compare the requested logical volume size with the available free size on the volume group. This is because in case of thin volumes, the available free size is based on the size of the corresponding thin-pool (with overprovsioning) instead of the available free size on the volume group.
- Command to create thin volumes:
``` sh
lvcreate -V 1G --thin -n thin_volume vg1/tpool
   WARNING: Sum of all thin volume sizes (1.00 GiB) exceeds the size of thin pool vg1/tpool and the size of whole volume group (992.00 MiB)!
   For thin pool auto extension activation/thin_pool_autoextend_threshold should be below 100.
   Logical volume “thin_volume” created.
```

#### LVMD Protocol changes

``` proto
+// Represents the details of thinpool.
+message ThinPoolItem {
+  // Data and Metadata fields are used for monitoring thinpool
+  double data_percent = 1; // Data percent occupied on the thinpool
+  double metadata_percent = 2; // Metadata percent occupied on the thinpool
+
+  // Used for scheduling decisions
+  uint64 overprovision_bytes = 3; // Free space on the thinpool with overprovision.
+}

// Represents the response corresponding to device class targets.
message WatchItem {
     uint64 free_bytes = 1; // Free space in the device class.
     string device_class = 2;
     uint64 size_bytes = 3; // Size of device class in bytes.
+    ThinPoolItem thin_pool = 4;
}

// Represents the stream output from Watch.
message WatchResponse {
    uint64 free_bytes = 1;  // Free space of the default volume group in bytes.
    repeated WatchItem items = 2;
}
```
- There are no changes to the API to Resize or Remove Logical Volumes as the underlying calls do not require the thinpool information, only the Logical Volume ID is required.
- Values for node annotations and monitoring will remain same if the device-class is targetting volume group.
- For thinpool targets:
    -  if the device-class having thinpool is default then `WatchResponse.free_bytes` will be same as `WatchItem.thin_pool.overprovision_bytes`.
    - `WatchItem.thin_pool` will have the values as stated in above listing
    - `WatchItem.free_bytes` will be empty for thinpool


#### Monitoring thin-pools
- We need to constantly monitor the thin-pools and ensure that it does not run out of space.
- `/etc/lvm/lvm.conf` settings can allow auto growth of the thin pool when required. By default, the threshold is 100% which means that the pool will not grow.
- If we set this to, 75%, the Thin Pool will autoextend when the pool is 75% full. It will increase by the default percentage of 20% if the value is not changed. We can see these settings using the command grep against the file.

```go=
# grep -E ‘^\s*thin_pool_auto’ /etc/lvm/lvm.conf
 thin_pool_autoextend_threshold = 100
 thin_pool_autoextend_percent = 20
```
- We assume that user has the threshold of 100% with no autogrow.
- Topolvm will regularly monitor the used thin-pool space for `data` and `metadata`.
- These usages will be exported as metrics so that admin/opeartor can analyse them and take precautionary steps when usage exeeds certain threshold limit.
- Command to be used for montoring:
``` go=
   lvs
   LV          VG   Attr       LSize   Pool  Origin Data%  Meta%  Move Log Cpy%Sync Convert
   lv1         vg1  -wi-ao---- 600.00m
   thin_volume vg1  Vwi-a-tz--   1.00g tpool        3.20
   tpool       vg1  twi-aotz-- 200.00m              16.38  1.37
```
- Topolvm will only monitor the thin-pools provided in the lvmd.yaml config file.
- Below metrics will be available under `thinpool` subsystem for thinpool device-class targets
    - `size_bytes`
    - `data_percent`
    - `metadata_percent`

#### Capacity awareness
- Topolvm uses capacity awareness to ensure that new pod is created on the node where available capacity of the volume group is larger than the requested capacity. `capacity.topolvm.cybozu.com/<device-class>` annotation is added on each node which consists of the available capacity on the volume group.
- Creation of thin logical volumes doesn't update the available device class capacity until the user actually uses storage on the thin volumes.
- So we need to ensure that all the thin logical volumes don't get created on the same node.
- This can be done by using the `overprovisionRatio`
- For device classes having thin pools, the value of `capacity.topolvm.cybozu.com/<device-class>` annotation will be calculated as:
  `(thin-pool-size * overprovisionedRatio)-(sum-of-all-thin-lv-virtual-sizes-in-thin-pool)`
- Note: Available thin pool on the volume groups will be obtained from lvmd.yaml config file.

## Open Questions
- Should topolvm create the thin-pools instead of the admin/operator?
    - Admin should create thin pools.

- If multiple thin-pools are supported for each volume group, then how to decide which thin-pool should be used to create the thin logical volumes.
    - We will support multiple thin pools per volume group by having multiple device classes mapping to a single volume group where each device class is mapped to a different thin pool within that volume group.

- How to do capacity awareness of thin-pools when Volume groups have both thick and thin logical volumes.
    - Multiple device classes can be mapped to a single volume group where each device class is mapped to a different thin pool within that volume group. Capacity of the device class with thin pools can be calculated using the overprovision ratio.

- Should we expect auto-extend of thin-pool to be enabled by default? What happens when the user has already enabled the autoextend feature when doing the monitoring?
    - No, it's not necessary.

- Does topolvm constantly poll the available capacity of the thin pools?
    - If there's no request for Logical Volume creation/expansion/removal, node annotations and metrics will be updated in 10 mins intervals.
    - If thinpool has been extended then user need to wait for a maximum of 10 mins to realise the thinpool extended size.

- How to reload lvmd to use updated lvmd.yaml?
    - Out of scope.
