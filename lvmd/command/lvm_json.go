package command

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/cybozu-go/log"
)

type vg struct {
	Name string `json:"vg_name"`
	UUID string `json:"vg_uuid"`
	Size uint64 `json:"vg_size"`
	Free uint64 `json:"vg_free"`
}

type lv struct {
	Name            string  `json:"lv_name"`
	FullName        string  `json:"lv_full_name"`
	UUID            string  `json:"lv_uuid"`
	Path            string  `json:"lv_path"`
	Major           int32   `json:"lv_kernel_major"` // might be negative, e.g., when LV is not active
	Minor           int32   `json:"lv_kernel_minor"` // might be negative, e.g., when LV is not active
	Origin          string  `json:"origin"`
	OriginSize      uint64  `json:"origin_size"`
	PoolLV          string  `json:"pool_lv"`
	Tags            string  `json:"lv_tags"`
	Attr            string  `json:"lv_attr"`
	VGName          string  `json:"vg_name"`
	Size            uint64  `json:"lv_size"`
	DataPercent     float64 `json:"data_percent"`
	MetaDataPercent float64 `json:"metadata_percent"`
}

func (u *lv) isThinPool() bool {
	return u.Attr[0] == 't'
}

type lvmFullReport struct {
	Report []lvmReport `json:"report"`
}

type lvmReport struct {
	VG []vg `json:"vg"`
	LV []lv `json:"lv"`
}

// Issue single lvm command that retrieves everything we need in one call and get the output as JSON
func getLVMState() ([]vg, []lv, error) {
	var stdout bytes.Buffer

	// Note: fullreport doesn't have an option to omit an entire section, so in those cases we limit
	// the column to 1 and then exclude it to remove.
	args := []string{
		"fullreport",
		"--config", "report {output_format=json pvs_cols_full=\"pv_name\" segs_cols_full=\"lv_uuid\" pvsegs_cols_full=\"lv_uuid\"} global {units=b suffix=0}",
		"--configreport", "pv", "-o-pv_name",
		"--configreport", "seg", "-o-lv_uuid",
		"--configreport", "pvseg", "-o-lv_uuid",
		"--configreport", "vg", "-o", "vg_name,vg_uuid,vg_size,vg_free",
		"--configreport", "lv", "-o", "lv_uuid,lv_name,lv_full_name,lv_path,lv_size," +
			"lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,lv_tags," +
			"lv_attr,vg_name,data_percent,metadata_percent,pool_lv",
	}

	c := wrapExecCommand(lvm, args...)
	c.Env = os.Environ()
	c.Env = append(c.Env, "LC_ALL=C")
	log.Info("invoking LVM command", map[string]interface{}{
		"args": args,
	})
	c.Stdout = &stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return nil, nil, err
	}

	var lfr lvmFullReport
	if err := json.Unmarshal(stdout.Bytes(), &lfr); err != nil {
		return nil, nil, err
	}

	var vgs []vg
	var lvs []lv
	for _, report := range lfr.Report {
		vgs = append(vgs, report.VG...)
		lvs = append(lvs, report.LV...)
	}
	return vgs, lvs, nil
}
