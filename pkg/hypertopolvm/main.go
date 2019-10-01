package main

import (
	"io"
	"os"
	"path/filepath"

	csitopolvm "github.com/cybozu-go/topolvm/pkg/csi-topolvm/cmd"
	lvmd "github.com/cybozu-go/topolvm/pkg/lvmd/cmd"
	lvmetrics "github.com/cybozu-go/topolvm/pkg/lvmetrics/cmd"
	controller "github.com/cybozu-go/topolvm/pkg/topolvm-controller/cmd"
	node "github.com/cybozu-go/topolvm/pkg/topolvm-node/cmd"
	scheduler "github.com/cybozu-go/topolvm/pkg/topolvm-scheduler/cmd"
)

func usage() {
	io.WriteString(os.Stderr, `Usage: hypertopolvm COMMAND [ARGS ...]

COMMAND:
    - csi-topolvm        Unified CSI driver.
    - lvmd               gRPC service to manage LVM volumes.
    - lvmetrics          DaemonSet sidecar container to expose storage metrics as Node annotations.
    - topolvm-scheduler  Scheduler extender.
    - topolvm-node       Sidecar to communicate with CSI controller over TopoLVM custom resources.
    - topolvm-controller Controllers for TopoLVM.
`)
}

func main() {
	name := filepath.Base(os.Args[0])
	if name == "hypertopolvm" {
		if len(os.Args) == 1 {
			usage()
			os.Exit(1)
		}
		name = os.Args[1]
		os.Args = os.Args[1:]
	}

	switch name {
	case "csi-topolvm":
		csitopolvm.Execute()
	case "lvmd":
		lvmd.Execute()
	case "lvmetrics":
		lvmetrics.Execute()
	case "topolvm-scheduler":
		scheduler.Execute()
	case "topolvm-node":
		node.Execute()
	case "topolvm-controller":
		controller.Execute()
	default:
		usage()
		os.Exit(1)
	}
}
