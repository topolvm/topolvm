package filesystem

import (
	"errors"
	"path/filepath"

	"golang.org/x/sys/unix"
)

var (
	// ErrUnsupportedFilesystem is an error for unsupported filesystems.
	ErrUnsupportedFilesystem = errors.New("unsupported filesystem")

	// ErrNonBlockDevice is an error returned when the file is not a block device.
	ErrNonBlockDevice = errors.New("not a block device")

	// ErrFilesystemExists is an error returned when the device has a filesystem.
	ErrFilesystemExists = errors.New("filesystem exists")

	fsTypeMap = make(map[string]func(device string) Filesystem)
)

// Filesystem represents the operations of a filesystem in a block device.
type Filesystem interface {
	// Exists returns true if the filesystem exists in the underlying block device.
	Exists() bool

	// Mkfs creates the filesystem on the underlying block device if the filesystem is not yet created.
	// This returns ErrFilesystemExists if device has some filesystem.
	Mkfs() error

	// Mount mounts the filesystem onto the target directory.
	// target directory must exist.
	// If device is mounted on target, this returns nil.
	// If readonly is true, the filesystem will be mounted as read-only.
	Mount(target string, readonly bool) error

	// Unmount unmounts if device is mounted.
	// target directory must exist.
	// If not mounted, this does nothing.
	Unmount(target string) error

	// Resize resizes the filesystem to match the size of the underlying block device.
	// It should be called while the filesystem is mounted.
	Resize(target string) error
}

// New returns a Filesystem if fsType is supported.
// device must be an existing block device file.
func New(fsType, device string) (Filesystem, error) {
	p, err := filepath.EvalSymlinks(device)
	if err != nil {
		return nil, err
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return nil, err
	}

	var st unix.Stat_t
	if err := Stat(p, &st); err != nil {
		return nil, err
	}
	if (st.Mode & unix.S_IFMT) != unix.S_IFBLK {
		return nil, ErrNonBlockDevice
	}

	newFS, ok := fsTypeMap[fsType]
	if !ok {
		return nil, ErrUnsupportedFilesystem
	}

	return newFS(p), nil
}
