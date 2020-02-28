package types

// Volume represents abstracted Volume.
type Volume struct {
	Node      string
	Name      string
	VolumeID  string
	RequestGb int64
	CurrentGb *int64
}

// GetCurrentGb returns value of currentGb and ok flag.
func (v *Volume) GetCurrentGb() (int64, bool) {
	if v.CurrentGb == nil {
		return 0, false
	}
	return *v.CurrentGb, true
}

// SetCurrentGb sets currentGb.
func (v *Volume) SetCurrentGb(n int64) {
	v.CurrentGb = &n
}
