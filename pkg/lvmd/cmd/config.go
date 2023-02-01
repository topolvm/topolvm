package cmd

import (
	"os"

	"github.com/cybozu-go/log"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/lvmd"
	"sigs.k8s.io/yaml"
)

// Config represents configuration parameters for lvmd
type Config struct {
	// SocketName is Unix domain socket name
	SocketName string `json:"socket-name"`
	// DeviceClasses is
	DeviceClasses []*lvmd.DeviceClass `json:"device-classes"`
	// LvcreateOptionClasses are classes that define options for the lvcreate command
	LvcreateOptionClasses []*lvmd.LvcreateOptionClass `json:"lvcreate-option-classes"`
}

var config = &Config{
	SocketName: topolvm.DefaultLVMdSocket,
}

func loadConfFile(cfgFilePath string) error {
	b, err := os.ReadFile(cfgFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(b, config)
	if err != nil {
		return err
	}
	log.Info("configuration file loaded: ", map[string]interface{}{
		"device_classes": config.DeviceClasses,
		"socket_name":    config.SocketName,
		"file_name":      cfgFilePath,
	})
	return nil
}
