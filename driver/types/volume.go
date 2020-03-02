package types

// Volume represents abstracted Volume.
type Volume struct {
	Node      string
	Name      string
	VolumeID  string
	RequestGb int64
	currentGb *int64
}

// GetCurrentGb returns value of currentGb and ok flag.
func (v *Volume) GetCurrentGb() (int64, bool) {
	if v.currentGb == nil {
		return 0, false
	}
	return *v.currentGb, true
}

// SetCurrentGb sets currentGb.
func (v *Volume) SetCurrentGb(n int64) {
	v.currentGb = &n
}
