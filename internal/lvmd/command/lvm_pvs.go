package command

import (
	"context"
	"encoding/json"
)

type pv struct {
	name string
}

func (u *pv) UnmarshalJSON(data []byte) error {
	type pvInternal struct {
		Name string `json:"pv_name"`
	}

	var temp pvInternal
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	u.name = temp.Name
	return nil
}

// ListPhysicalVolumes returns the device paths (pv_name) of the physical
// volumes backing the volume group, e.g. ["/dev/nvme0n1"].
func (vg *VolumeGroup) ListPhysicalVolumes(ctx context.Context) ([]string, error) {
	type pvReport struct {
		Report []struct {
			PV []pv `json:"pv"`
		} `json:"report"`
	}
	res := new(pvReport)
	args := []string{
		"pvs", "-o", "pv_name", "--select", "vg_name=" + vg.Name(),
		"--reportformat", "json",
	}
	if err := callLVMInto(ctx, res, verbosityLVMStateNoUpdate, args...); err != nil {
		return nil, err
	}

	var paths []string
	for _, report := range res.Report {
		for _, p := range report.PV {
			if p.name != "" {
				paths = append(paths, p.name)
			}
		}
	}
	return paths, nil
}
