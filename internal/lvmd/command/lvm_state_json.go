package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func parseFullReportResult(data io.ReadCloser) ([]vg, []lv, error) {
	type fullReportResult struct {
		Report []struct {
			VG []vg `json:"vg"`
			LV []lv `json:"lv"`
		} `json:"report"`
	}

	var result fullReportResult
	if err := json.NewDecoder(data).Decode(&result); err != nil {
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
	streamed, err := callLVMStreamed(ctx, verbosityLVMStateNoUpdate, append([]string{"fullreport"}, args...)...)
	defer func() {
		// this will wait for the process to be released.
		if err := streamed.Close(); err != nil {
			log.FromContext(ctx).Error(err, "failed to run command")
		}
	}()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute command: %v", err)
	}
	return parseFullReportResult(streamed)
}
