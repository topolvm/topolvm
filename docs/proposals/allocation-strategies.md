# Allocation strategies

<!-- toc -->

<!-- /toc -->

## Summary

Allocation strategies is a feature to allow choosing how to allocate LVs from the VG when using TopoLVM.
For example, keep the LV on a single PV or stripe it over multiple.

## Motivation

Some applications benefit from "owning" the whole disk they use and keeping the data on a single physical disk.
For optimal performance, this physical disk should not be shared with other applications.

### Goals

- Make it possible to reserve a whole PV for one LV (`dedicated`).
- Expose the `--alloc` flag of `lvcreate` to the TopoLVM user.
- Keep backward compatibility.

## Proposal

### Option A) lvmd options

Specify additional options for how to create the LVs in the `lvmd` config.
This is similar to how striping is currently configured.
Each device-class can have different options and end users will select a StorageClass based on what options they want.

```yaml
device-classes:
  # The new options are not required
  - name: ssd
    volume-group: ssd-vg
    spare-gb: 10
    default: true
  # Ask for dedicated PV for every volume in this device-class
  - name: ssd-dedicated
    volume-group: dedicated-ssd-vg
    spare-gb: 10
    dedicated: true
  # Specify the allocation policy: contiguous|cling|cling_by_tags|normal|anywhere|inherit
  - name: ssd-contiguous
    volume-group: contiguous-ssd-vg
    spare-gb: 10
    allocation: contiguous
  # Striped works as before and can be combined with "allocation"
  - name: striped
    volume-group: multi-pv-vg
    spare-gb: 10
    stripe: 2
    stripe-size: "64"
```

Pros:
- Similar to how striping is already implemented.
- Changes are only required for `lvmd`.

Cons:
- Will require one (pre-configured) device-class and StorageClass for each way of creating LVs.

### Option B) additional parameters passed to StorageClass

Similar to A) but instead of using the `lvmd` config, the options are passed in as parameters on the StorageClass.


```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/device-class": "ssd"
  "topolvm.cybozu.com/allocation": "contiguous"
  "topolvm.cybozu.com/dedicated": false
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Pros:
- Easier to provide as self-service since all that is required is access to StorageClasses.

Cons:
- Will require larger changes (e.g the `lvmd` protocol must support the options).

### Decision outcome

TODO

## Design details

The `allocation` option should simply be passed on to `lvcreate` as the `--alloc` flag.

The `dedicated` option would indicate that each LV should be created on its own dedicated PV.
This could be implemented by adjusting the size of the volume up to fill out the whole PV.

1. Find a large enough PV that is not currently used.
2. Adjust the size of the request to fill the PV.
3. Use `lvcreate` with the PV positional argument to indicate what PV to use.

### Scheduling

Dedicated volumes makes capacity based scheduling harder since it is no longer enough to look at the free capacity of the VG.
The same is true for volumes with `contiguous` allocation policy.

Optimal scheduling of these types of volumes can be considered as "finding the smallest PV that can take the LV", since this means wasting as little disk space as possible.
Unfortunately this means that it is not possible to achieve optimal scheduling based on capacity alone.
However, if we assume that the PVs are uniform in size, capacity based scheduling will still work fine.
This assumption is not completely unreasonable, as it is quite common to have multiple disks of the same size, especially in a data center.
There could still be problems if a single VG was used with different allocation strategies (e.g. both striped and contiguous LVs), but since 2 device-classes are not allowed to target the same VG, we can assume that all LVs in a VG will use the same allocation policy.

Below are some thoughts on strategies for improving capacity based scheduling in specific situations.

#### Largest possible dedicated volume

Scheduling decisions for dedicated volumes could be based on the largest unused PV.
If there is no unused PV, the available capacity of that Node is 0 for dedicated volumes.
If there are multiple unused PVs, the available capacity of the Node is the same as the capacity of the largest unused PV.
This strategy will correctly indicate the largest possible dedicated volume for each Node, but it will favor Nodes with larger unused PVs over Nodes with smaller even though it would be more efficient to find the smallest unused PV with enough capacity.

#### Mean unused PV size and number

Another alternative would be to use the mean capacity of the unused PVs.
This metric would help favor Nodes with many large PVs (as opposed to only the largest), but it would not correctly indicate the largest possible dedicated volume.
This means that there is a risk that the scheduler finds no suitable Node even though there exists Nodes with large enough PVs.

#### Scheduling for other types of volumes

Similar calculations could be made for other allocation policies also.
For example a device-class with `allocation` set to `contiguous` would require that there is a single PV with enough free capacity to hold the LV.
This is the same as for dedicated volumes, except that the PV is allowed to be shared with other LVs.

#### Conclusion

The strategies discussed above does not provide any silver bullet solution.
Therefore, to avoid complicating things too much, it is suggested that no special scheduling is implemented at this point.
Normal capacity based scheduling is assumed to work adequately given that PVs belonging to the same device-class are uniform in size.

### Should multiple device-classes be able to target the same VG?

Currently it is not allowed to have two device-classes target the same VG.
This means that a separate VG is needed for each option that should be available as a device-class.
For example, it is not possible to use the same VG for both striped and linear LVs.
We may want change this in the future so that it is possible to use the same VG with different options.
There is one problem however, the `spare-gb` must then be the same for each device-class targeting the same VG.
Allowing multiple device-classes targeting the same VG will also complicate scheduling *if* other factors than capacity are to be considered.

The suggestion is to keep the current behavior for now.
