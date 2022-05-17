# Allocation strategies

## Summary

LVCreate options is a feature to allow passing options to `lvcreate` when using TopoLVM.
For example, set the `alloc` flag or specify a `raid` configuration.

## Motivation

LVM offers many options for how to create an LV (e.g. RAID configurations and mirroring).
These options would be useful to expose through TopoLVM so users can make use of them.

### Goals

- Make it possible to pass `lvcreate` options through TopoLVM.
- Keep backward compatibility.

## Proposal

### Option A) lvmd options

Specify additional options for how to create the LVs in the `lvmd` config.
Each device-class can have different options and end users will select a StorageClass based on what options they want.

```yaml
device-classes:
  # The new options are not required
  - name: ssd
    volume-group: ssd-vg
    spare-gb: 10
    default: true
  # Specify allocation policy
  - name: ssd-contiguous
    volume-group: contiguous-ssd-vg
    spare-gb: 10
    lvcreate-options:
      - --alloc contiguous
  # Specify raid configuration
  - name: ssd-contiguous
    volume-group: contiguous-ssd-vg
    spare-gb: 10
    lvcreate-options:
      - --type raid10
      - --mirrors 2
  # Striped works as before and can be combined with extra options
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
provisioner: topolvm.io
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.io/device-class": "ssd"
  "topolvm.io/lvcreate-options": "--type raid10 --mirrors 2"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

Pros:
- Easier to provide as self-service since all that is required is access to StorageClasses.

Cons:
- Will require larger changes (e.g the `lvmd` protocol must support the options).

### Decision outcome

Option A) lvmd options was chosen since it requires fewer changes and is easier for administrators to understand.
As suggested below it was also decided to not make changes to how scheduling works or allow multiple device-classes to target the same VG.

## Design details

Option A) would make the current implementation of striping a special case that is perhaps a bit odd.
To unify and make all `lvcreate` options equal, we could consider deprecating the `stripe` and `stripe-size` fields in the `lvmd` config.
Striping could then be set through `lvcreate-options` like all other options.
However, this would be a breaking change, so for now it is suggested to keep the fields.

### Scheduling

Scheduling decisions are not trivial when considering all possible options for `lvcreate`.
Some options require multiple PVs (e.g. RAID configurations and mirroring), other require that the LV should not be spread across multiple PVs (e.g. `--alloc contiguous`).
If all of options must be considered when making scheduling decisions it would make scheduling very complicated.
Therefore it is suggested to *not* change the scheduling.

It would instead be the responsibility of the administrator that installs TopoLVM to make sure that device classes and VGs are created in a way that makes sense.
This means that the administrator should, for example, make sure that a VG that will be used with `--mirrors 2` must have at least 2 PVs.

### Should multiple device-classes be able to target the same VG?

Currently it is not allowed to have two device-classes target the same VG.
This means that a separate VG is needed for each option that should be available as a device-class.
For example, it is not possible to use the same VG for both striped and linear LVs.
We may want change this in the future so that it is possible to use the same VG with different options.
There is one problem however, the `spare-gb` must then be the same for each device-class targeting the same VG.
Allowing multiple device-classes targeting the same VG will also complicate scheduling *if* other factors than capacity are to be considered.

The suggestion is to keep the current behavior for now.
