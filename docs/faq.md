Frequently Asked Questions
==========================

## Why does `lvmd` run on the host OS?

Because LVM is not designed for containers.

For example, LVM commands need an exclusive lock to avoid conflicts.
If the same PV/VG/LV is shared between host OS and containers, the commands would conflict.
In the worst case, the metadata will be corrupted.

If `lvmd` can use storage devices exclusively, it might be able to create
PV/VG/LV using those devices in containers.  This option is not implemented, though.
