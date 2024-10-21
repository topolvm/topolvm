package app

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Health check for lvmd server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return healthSubMain(cmd.Context(), config)
	},
}

func healthSubMain(ctx context.Context, config *Config) error {
	err := loadConfFile(ctx, cfgFilePath)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(
		"unix:"+config.SocketName,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	client := grpc_health_v1.NewHealthClient(conn)

	res, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if status := res.GetStatus(); status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("lvmd does not working: %s", status.String())
	}
	return nil
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
