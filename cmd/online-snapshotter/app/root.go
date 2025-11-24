package app

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var rootCmd = &cobra.Command{
	Use:     "online-snapshotter",
	Version: topolvm.Version,
	Short:   "TopoLVM online snapshot and restore tool",
	Long:    `online-snapshotter provides File System Snapshot and Restore features for TopoLVM using Restic or Kopia.`,
}

func init() {
	ctrl.SetLogger(zap.New())
	rootCmd.AddCommand(newBackupCommand())
	rootCmd.AddCommand(newRestoreCommand())
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
