package command

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

type vg struct {
	name string
	uuid string
	size uint64
	free uint64
}

type lv struct {
	name            string
	fullName        string
	uuid            string
	path            string
	major           uint64
	minor           uint64
	origin          string
	originSize      uint64
	poolLV          string
	tags            []string
	attr            string
	vgName          string
	size            uint64
	dataPercent     float64
	metaDataPercent float64
}

func (u *lv) isThinPool() bool {
	return u.attr[0] == 't'
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
	u.size, convErr = strconv.ParseUint(temp.Size, 10, 64)
	if convErr != nil {
		return convErr
	}
	u.free, convErr = strconv.ParseUint(temp.Free, 10, 64)
	if convErr != nil {
		return convErr
	}

	return nil
}

func (u *lv) UnmarshalJSON(data []byte) error {
	type lvInternal struct {
		Name            string `json:"lv_name"`
		FullName        string `json:"lv_full_name"`
		UUID            string `json:"lv_uuid"`
		Path            string `json:"lv_path"`
		Major           string `json:"lv_kernel_major"`
		Minor           string `json:"lv_kernel_minor"`
		Origin          string `json:"origin"`
		OriginSize      string `json:"origin_size"`
		PoolLV          string `json:"pool_lv"`
		Tags            string `json:"lv_tags"`
		Attr            string `json:"lv_attr"`
		VgName          string `json:"vg_name"`
		Size            string `json:"lv_size"`
		DataPercent     string `json:"data_percent"`
		MetaDataPercent string `json:"metadata_percent"`
	}

	var temp lvInternal
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	u.name = temp.Name
	u.fullName = temp.FullName
	u.uuid = temp.UUID
	u.path = temp.Path

	// If LV is not active, major/minor numbers will be -1, ignore conversion
	// errors which results in 0 for values in this case.
	u.major, _ = strconv.ParseUint(temp.Major, 10, 32)
	u.minor, _ = strconv.ParseUint(temp.Minor, 10, 32)

	var convErr error
	u.origin = temp.Origin
	if len(temp.OriginSize) > 0 {
		u.originSize, convErr = strconv.ParseUint(temp.OriginSize, 10, 64)
		if convErr != nil {
			return convErr
		}
	}

	u.poolLV = temp.PoolLV
	u.tags = strings.Split(temp.Tags, ",")
	u.attr = temp.Attr
	u.vgName = temp.VgName

	if len(temp.Size) > 0 {
		u.size, convErr = strconv.ParseUint(temp.Size, 10, 64)
		if convErr != nil {
			return convErr
		}
	}

	if len(temp.DataPercent) > 0 {
		u.dataPercent, convErr = strconv.ParseFloat(temp.DataPercent, 64)
		if convErr != nil {
			return convErr
		}
	}

	if len(temp.MetaDataPercent) > 0 {
		u.metaDataPercent, convErr = strconv.ParseFloat(temp.MetaDataPercent, 64)
		if convErr != nil {
			return convErr
		}
	}
	return nil
}

func parseFullReportResult(data []byte) ([]vg, []lv, error) {
	type fullReportResult struct {
		Report []struct {
			VG []vg `json:"vg"`
			LV []lv `json:"lv"`
		} `json:"report"`
	}

	var result fullReportResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil, err
	}

	var vgs []vg
	var lvs []lv
	for _, report := range result.Report {
		vgs = append(vgs, report.VG...)
		lvs = append(lvs, report.LV...)
	}
	return vgs, lvs, nil
}

// Issue single lvm command that retrieves everything we need in one call and get the output as JSON
func getLVMState(ctx context.Context) ([]vg, []lv, error) {
	args := []string{
		"--reportformat", "json",
		"--units", "b", "--nosuffix",
		"--configreport", "vg", "-o", "vg_name,vg_uuid,vg_size,vg_free",
		"--configreport", "lv", "-o", "lv_uuid,lv_name,lv_full_name,lv_path,lv_size," +
			"lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,lv_tags," +
			"lv_attr,vg_name,data_percent,metadata_percent,pool_lv",
		// fullreport doesn't have an option to omit an entire section, so we
		// omit all fields instead.
		"--configreport", "pv", "-o,",
		"--configreport", "pvseg", "-o,",
		"--configreport", "seg", "-o,",
	}
	stdout, err := callLVMWithStdout(ctx, "fullreport", args...)
	if err != nil {
		return nil, nil, err
	}

	return parseFullReportResult(stdout)
}
