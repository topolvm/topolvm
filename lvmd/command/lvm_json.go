package command

import (
	"encoding/json"
	"io"
	"os"
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

func parseLVMJSON(data []byte) (vgs []vg, lvs []lv, _ error) {
	type lvmResponse struct {
		Report json.RawMessage `json:"report"`
		Log    json.RawMessage `json:"log"`
	}

	type lvmEntries struct {
		Vg json.RawMessage `json:"vg"`
		Lv json.RawMessage `json:"lv"`
	}

	var result lvmResponse

	if parseErr := json.Unmarshal(data, &result); parseErr != nil {
		return nil, nil, parseErr
	}

	if result.Report != nil {
		var entries []lvmEntries
		if parseErr := json.Unmarshal(result.Report, &entries); parseErr != nil {
			return nil, nil, parseErr
		}

		for _, e := range entries {
			if e.Vg != nil {
				var tvgs []vg
				if parseErr := json.Unmarshal(e.Vg, &tvgs); parseErr != nil {
					return nil, nil, parseErr
				}
				vgs = append(vgs, tvgs...)
			}
			if e.Lv != nil {
				var tlvs []lv
				if parseErr := json.Unmarshal(e.Lv, &tlvs); parseErr != nil {
					return nil, nil, parseErr
				}
				lvs = append(lvs, tlvs...)
			}
		}
	}
	return vgs, lvs, nil
}

// Issue single lvm command that retrieves everything we need in one call and get the output as JSON
func getLVMState() (vgs []vg, lvs []lv, _ error) {

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
	c.Stderr = os.Stderr
	stdout, err := c.StdoutPipe()
	if err != nil {
		return vgs, lvs, err
	}
	if err := c.Start(); err != nil {
		return nil, nil, err
	}
	out, err := io.ReadAll(stdout)
	if err != nil {
		return nil, nil, err
	}
	if err := c.Wait(); err != nil {
		return nil, nil, err
	}

	return parseLVMJSON(out)
}
