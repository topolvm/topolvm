package command

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

type vg struct {
	name string
	uuid string
	size uint64
	free uint64
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

func getVGReport(ctx context.Context, name string) (vg, error) {
	type vgReport struct {
		Report []struct {
			VG []vg `json:"vg"`
		} `json:"report"`
	}
	res := new(vgReport)
	args := []string{
		"vgs", name, "-o", "vg_uuid,vg_name,vg_size,vg_free", "--units", "b", "--nosuffix", "--reportformat", "json",
	}
	err := callLVMInto(ctx, res, args...)

	if IsLVMNotFound(err) {
		return vg{}, errors.Join(ErrNotFound, err)
	}

	if err != nil {
		return vg{}, err
	}

	for _, report := range res.Report {
		for _, vg := range report.VG {
			if vg.name == name {
				return vg, nil
			}
		}
	}

	return vg{}, ErrNotFound
}
