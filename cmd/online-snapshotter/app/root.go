package app

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var rootCmd = &cobra.Command{
	Use:     topolvmv1.TopoLVMSnapshotter,
	Version: topolvm.Version,
	Short:   "TopoLVM online snapshot and restore tool",
	Long:    `topolvm-snapshotter provides File System Snapshot and Restore features for TopoLVM using Restic or Kopia.`,
}

func init() {
	ctrl.SetLogger(zap.New())
	rootCmd.AddCommand(newBackupCommand())
	rootCmd.AddCommand(newRestoreCommand())
	rootCmd.AddCommand(newDeleteCommand())
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
