package app

import (
	"context"
	"os"

	"github.com/topolvm/topolvm"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// Config represents configuration parameters for lvmd
type Config struct {
	// SocketName is Unix domain socket name
	SocketName string `json:"socket-name"`
	// DeviceClasses is
	DeviceClasses []*lvmdTypes.DeviceClass `json:"device-classes"`
	// LvcreateOptionClasses are classes that define options for the lvcreate command
	LvcreateOptionClasses []*lvmdTypes.LvcreateOptionClass `json:"lvcreate-option-classes"`
}

var config = &Config{
	SocketName: topolvm.DefaultLVMdSocket,
}

func loadConfFile(ctx context.Context, cfgFilePath string) error {
	b, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, config)
	if err != nil {
		return err
	}
	log.FromContext(ctx).Info("configuration file loaded",
		"device_classes", config.DeviceClasses,
		"socket_name", config.SocketName,
		"file_name", cfgFilePath,
	)
	return nil
}
