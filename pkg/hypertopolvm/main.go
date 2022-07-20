package main

import (
	"io"
	"os"
	"path/filepath"

	lvmd "github.com/topolvm/topolvm/pkg/lvmd/cmd"
	controller "github.com/topolvm/topolvm/pkg/topolvm-controller/cmd"
	node "github.com/topolvm/topolvm/pkg/topolvm-node/cmd"
	scheduler "github.com/topolvm/topolvm/pkg/topolvm-scheduler/cmd"
)

func usage() {
	io.WriteString(os.Stderr, `Usage: hypertopolvm COMMAND [ARGS ...]

COMMAND:
    topolvm-controller:  TopoLVM CSI controller service.
    topolvm-node:        TopoLVM CSI node service.
    topolvm-scheduler:   Scheduler extender.
    lvmd:                gRPC service to manage LVM volumes.
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
	case "lvmd":
		lvmd.Execute()
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
