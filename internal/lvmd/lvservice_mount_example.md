# MountLV RPC Implementation

## Overview
The `MountLV` RPC has been implemented to mount logical volumes to specific directories with full filesystem management.

## Features Implemented

### 1. Filesystem Detection
- Uses `filesystem.DetectFilesystem()` to check if the device already has a filesystem
- Validates that existing filesystem matches the requested type
- Returns an error if there's a mismatch to prevent data loss

### 2. Filesystem Formatting
- Automatically formats the device if no filesystem exists
- Supports multiple filesystem types:
  - **ext4** (default)
  - **ext3**
  - **xfs**
- Uses appropriate formatting commands:
  - `mkfs.ext4 -F <device>` for ext4
  - `mkfs.ext3 -F <device>` for ext3
  - `mkfs.xfs -f <device>` for xfs

### 3. Mount Operations
- Creates the target directory if it doesn't exist (using `os.MkdirAll`)
- Checks if the target is already mounted (using `mountpoint -q`)
- Executes the mount command with proper options:
  ```bash
  mount -o <options> -t <fstype> <device> <target>
  ```
- Sets appropriate permissions (0777 with setgid) on the mounted directory

### 4. XFS Special Handling
- Automatically adds the `nouuid` option for XFS filesystems
- This is important for mounting XFS snapshots or cloned volumes

## RPC Definition

```protobuf
message MountLVRequest {
    string name = 1;                        // The logical volume name
    string device_class = 2;                // Device class of the LV
    string target_path = 3;                 // The directory path where the LV should be mounted
    string fs_type = 4;                     // Filesystem type (e.g., ext4, xfs). Defaults to ext4
    repeated string mount_options = 5;      // Mount options (e.g., rw, ro, noatime, etc.)
}

message MountLVResponse {
    string device_path = 1;                 // Path to the mounted device
}
```

## Usage Example

### gRPC Call
```go
req := &proto.MountLVRequest{
    Name:         "my-volume",
    DeviceClass:  "default",
    TargetPath:   "/mnt/mydata",
    FsType:       "ext4",
    MountOptions: []string{"rw", "noatime"},
}

resp, err := lvService.MountLV(ctx, req)
if err != nil {
    log.Fatalf("Failed to mount: %v", err)
}
log.Printf("Mounted device: %s", resp.DevicePath)
```

## Implementation Details

### Error Handling
- Returns `NotFound` if device class or logical volume doesn't exist
- Returns `InvalidArgument` if target_path is empty or filesystem type is unsupported
- Returns `FailedPrecondition` if device has a different filesystem than requested
- Returns `Internal` for system errors (mkdir, format, mount failures)

### Idempotency
- Checks if the target is already mounted before attempting to mount
- Returns success if already mounted (idempotent behavior)

### Commands Executed

1. **Directory Creation**: `os.MkdirAll(targetPath, 0755)`
2. **Filesystem Detection**: `/sbin/blkid -p -u filesystem -o export <device>`
3. **Format (if needed)**: `mkfs.<fstype> -F/-f <device>`
4. **Mount Check**: `mountpoint -q <target>`
5. **Mount**: `mount -o <options> -t <fstype> <device> <target>`
6. **Permissions**: `chmod 2777 <target>`

## Testing Considerations

To test this RPC, you'll need:
1. A working LVM setup with volume groups
2. Root privileges (mounting requires root)
3. Filesystem utilities installed (mkfs.ext4, mkfs.xfs, etc.)

### Example Test Scenario
```bash
# 1. Create a logical volume (using existing CreateLV RPC)
# 2. Call MountLV to mount it
# 3. Verify mount with: mount | grep /mnt/mydata
# 4. Verify filesystem with: df -hT /mnt/mydata
```

## Security Considerations

- The RPC requires appropriate permissions to execute mount operations
- Target paths should be validated to prevent mounting to sensitive locations
- Consider adding path allowlists in production environments

