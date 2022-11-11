package cmd

import (
	"context"
	"fmt"
	"net"

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
		return healthSubMain(config)
	},
}

func healthSubMain(config *Config) error {
	err := loadConfFile(cfgFilePath)
	if err != nil {
		return err
	}
	dialer := &net.Dialer{}
	dialFunc := func(ctx context.Context, a string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", a)
	}
	conn, err := grpc.Dial(config.SocketName, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialFunc))
	if err != nil {
		return err
	}
	defer conn.Close()
	client := grpc_health_v1.NewHealthClient(conn)

	ctx := context.Background()
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
