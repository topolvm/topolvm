package command

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

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

func getLVReport(ctx context.Context, name string) (map[string]lv, error) {
	type lvReport struct {
		Report []struct {
			LV []lv `json:"lv"`
		} `json:"report"`
	}

	var res = new(lvReport)

	args := []string{
		"lvs",
		name,
		"-o",
		"lv_uuid,lv_name,lv_full_name,lv_path,lv_size," +
			"lv_kernel_major,lv_kernel_minor,origin,origin_size,pool_lv,lv_tags," +
			"lv_attr,vg_name,data_percent,metadata_percent,pool_lv",
		"--units",
		"b",
		"--nosuffix",
		"--reportformat",
		"json",
	}
	err := callLVMInto(ctx, res, args...)

	if IsLVMNotFound(err) {
		return nil, errors.Join(ErrNotFound, err)
	}

	if err != nil {
		return nil, err
	}

	if len(res.Report) == 0 {
		return nil, ErrNotFound
	}

	lvs := res.Report[0].LV

	if len(lvs) == 0 {
		return nil, ErrNotFound
	}

	lvmap := make(map[string]lv, len(lvs))
	for _, lv := range lvs {
		lvmap[lv.name] = lv
	}

	return lvmap, nil
}
