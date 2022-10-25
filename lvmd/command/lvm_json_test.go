package command

import (
	"os"
	"testing"

	"github.com/topolvm/topolvm/lvmd/testutils"
)

func TestLvmJSON(t *testing.T) {
	goodJSON := `
	  {
		"report": [
		  {
			"vg": [
			  {
				"vg_name": "myvg1",
				"vg_uuid": "P8en82-LNUe-MERd-mOTT-XlAS-fkp8-1bleiB",
				"vg_size": "2199014866944",
				"vg_free": "2198482190336"
			  }
			],
			"pv": [
			  {},
			  {}
			],
			"lv": [
			  {
				"lv_uuid": "n3eoy5-R1B3-9S6A-rBwo-3n9f-mIxA-Dy4nnw",
				"lv_name": "thinpool",
				"lv_full_name": "myvg1/thinpool",
				"lv_path": "",
				"lv_size": "524288000",
				"lv_kernel_major": "253",
				"lv_kernel_minor": "2",
				"origin": "",
				"origin_size": "",
				"pool_lv": "",
				"lv_tags": "some_tag,some_tag2",
				"lv_attr": "twi-a-tz--",
				"vg_name": "myvg1",
				"data_percent": "0.00",
				"metadata_percent": "10.84"
			  }
			],
			"pvseg": [
			  {},
			  {},
			  {},
			  {},
			  {}
			],
			"seg": [
			  {}
			]
		  }
		]
	  }
	`
	vgs, lvs, err := parseFullReportResult([]byte(goodJSON))

	if err != nil {
		t.Fatal(err)
	}

	if vgs == nil {
		t.Fatal("vgs unexpectedly nil")
	}

	if lvs == nil {
		t.Fatal("lvs unexpectedly nil")
	}

	if len(lvs) != 1 {
		t.Fatal("Incorrect number of LVs returned: ", len(lvs))
	}

	if len(vgs) != 1 {
		t.Fatal("Incorrect number of VGs returned: ", len(vgs))
	}

	lv := lvs[0]

	if lv.uuid != "n3eoy5-R1B3-9S6A-rBwo-3n9f-mIxA-Dy4nnw" {
		t.Fatal("Incorrect uuid: ", lv.uuid)
	}

	if lv.name != "thinpool" {
		t.Fatal("Incorrect name: ", lv.name)
	}

	if lv.fullName != "myvg1/thinpool" {
		t.Fatal("Incorrect full name: ", lv.fullName)
	}

	if lv.path != "" {
		t.Fatal("Incorrect path: ", lv.path)
	}

	if lv.size != 524288000 {
		t.Fatal("Incorrect size: ", lv.size)
	}

	if lv.major != 253 {
		t.Fatal("Incorrect major number:", lv.major)
	}

	if lv.minor != 2 {
		t.Fatal("Incorrect minor number:", lv.minor)
	}

	if lv.origin != "" {
		t.Fatal("Incorrect origin:", lv.origin)
	}

	if lv.originSize != 0 {
		t.Fatal("Incorrect origin size:", lv.originSize)
	}

	if lv.poolLV != "" {
		t.Fatal("Incorrect pool lv:", lv.poolLV)
	}

	if len(lv.tags) != 2 {
		t.Fatal("Incorrect number of tags:", len(lv.tags))
	}

	if lv.tags[0] != "some_tag" || lv.tags[1] != "some_tag2" {
		t.Fatal("Incorrect tags:", lv.tags)
	}

	if lv.attr != "twi-a-tz--" {
		t.Fatal("Incorrect attr data: ", lv.attr)
	}

	if lv.vgName != "myvg1" {
		t.Fatal("Incorrect VG name: ", lv.vgName)
	}

	if lv.dataPercent != 0 {
		t.Fatal("Incorrect data percent: ", lv.dataPercent)
	}

	if lv.metaDataPercent != 10.84 {
		t.Fatal("Incorrect meta data percent:", lv.metaDataPercent)
	}

	vg := vgs[0]
	if vg.name != "myvg1" {
		t.Fatal("Incorrect vg.name: ", vg.name)
	}

	if vg.uuid != "P8en82-LNUe-MERd-mOTT-XlAS-fkp8-1bleiB" {
		t.Fatal("Incorrect vg.uuid: ", vg.uuid)
	}

	if vg.size != 2199014866944 {
		t.Fatal("Incorrect vg.size: ", vg.size)
	}

	if vg.free != 2198482190336 {
		t.Fatal("Incorrect vg.free: ", vg.free)
	}
}

func TestLvmInactiveMajorMinor(t *testing.T) {
	inactiveMajorMinor := `
	{
	  "report": [
		{
		  "vg": [
			{
			  "vg_name": "myvg1",
			  "vg_uuid": "P8en82-LNUe-MERd-mOTT-XlAS-fkp8-1bleiB",
			  "vg_size": "2199014866944",
			  "vg_free": "2198482190336"
			}
		  ],
		  "pv": [
			{},
			{}
		  ],
		  "lv": [
			{
			  "lv_uuid": "n3eoy5-R1B3-9S6A-rBwo-3n9f-mIxA-Dy4nnw",
			  "lv_name": "thinpool",
			  "lv_full_name": "myvg1/thinpool",
			  "lv_path": "",
			  "lv_size": "524288000",
			  "lv_kernel_major": "-1",
			  "lv_kernel_minor": "-1",
			  "origin": "",
			  "origin_size": "",
			  "pool_lv": "",
			  "lv_tags": "some_tag,some_tag2",
			  "lv_attr": "twi-a-tz--",
			  "vg_name": "myvg1",
			  "data_percent": "0.00",
			  "metadata_percent": "10.84"
			}
		  ],
		  "pvseg": [
			{},
			{},
			{},
			{},
			{}
		  ],
		  "seg": [
			{}
		  ]
		}
	  ]
	}
  `
	vgs, lvs, err := parseFullReportResult([]byte(inactiveMajorMinor))

	if err != nil {
		t.Fatal(err)
	}

	if vgs == nil {
		t.Fatal("vgs unexpectedly nil")
	}

	if lvs == nil {
		t.Fatal("lvs unexpectedly nil")
	}

	if len(lvs) != 1 {
		t.Fatal("Incorrect number of LVs returned: ", len(lvs))
	}

	if len(vgs) != 1 {
		t.Fatal("Incorrect number of VGs returned: ", len(vgs))
	}

	lv := lvs[0]

	if lv.major != 0 {
		t.Fatal("Incorrect major number:", lv.major)
	}

	if lv.minor != 0 {
		t.Fatal("Incorrect minor number:", lv.minor)
	}
}

func TestLvmJSONBad(t *testing.T) {
	truncatedJSON := `
	  {
		"report": [
		  {
			"vg": [
			  {
				"vg_name": "myvg1",
				"vg_uuid": "P8en82-LNUe-MERd-mOTT-XlAS-fkp8-1bleiB",
				"vg_size": "2199014866944",
				"vg_free": "2198482190336"
			  }
			],
			"pv": [
			  {},
			  {}
			],
			"lv": [
	`
	vgs, lvs, err := parseFullReportResult([]byte(truncatedJSON))

	if vgs != nil {
		t.Fatal("Expected vgs to be nil!")
	}

	if lvs != nil {
		t.Fatal("Expected lvs to be nil")
	}

	if err == nil {
		t.Fatal("Expected an err, got none")
	}

}

func TestLvmRetrieval(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}

	vgName := "test_lvm_json"
	lvName := "test_lvm_json_lv"
	loop, err := testutils.MakeLoopbackDevice(vgName)
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(vgName, loop)
	if err != nil {
		t.Fatal(err)
	}

	defer testutils.CleanLoopbackVG(vgName, []string{loop}, []string{vgName})

	vgs, lvs, err := getLVMState()

	if err != nil {
		t.Fatal("Unexpected err returned: ", err)
	}

	if lvs != nil {
		t.Fatal("Expected lvs to be nil")
	}

	if len(vgs) != 1 {
		t.Fatal("Expected 1 vg: ", len(vgs))
	}

	err = testutils.MakeLoopbackLV(lvName, vgName)
	if err != nil {
		t.Fatal(err)
	}

	vgs, lvs, err = getLVMState()
	if err != nil {
		t.Fatal(err)
	}

	if lvs == nil {
		t.Fatal("Expected LVs to exist")
	}

	if len(lvs) != 1 {
		t.Fatal("Expected 1 LV to exist")
	}

	vg := vgs[0]
	if vg.name != vgName {
		t.Fatal("Incorrect vg name: ", vg.name)
	}

	lv := lvs[0]
	if lv.name != lvName {
		t.Fatal("Incorrect lv name: ", lv.name)
	}
}
