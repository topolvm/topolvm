package cmd

import (
	"context"
	"fmt"
	"os"
	"net"

	"github.com/cybozu-go/well"
	"github.com/cybozu-go/topolvm/lvmd/proto"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

var config struct {
	socketName     string
	nodename       string
}

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lvmetrics",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

func subMain() error {
	err := well.LogConfig{}.Apply()
	if err != nil {
		return err
	}

	dialer := func(ctx context.Context, a string) (net.Conn, error) {
		return net.Dial("unix", a)
	}
	conn, err := grpc.Dial(config.socketName, grpc.WithInsecure(), grpc.WithContextDialer(dialer))
	defer conn.Close()



	well.Go(func(ctx context.Context) error {
		client := proto.NewVGServiceClient(conn)
		wClient, err := client.Watch(ctx, &proto.Empty{})
		if err != nil {
			return err
		}

		for {
			res, err := wClient.Recv()
			if err != nil {
				return err
			}
		}

		return nil
	})

	well.Stop()
	err = well.Wait()
	if err != nil && !well.IsSignaled(err) {
		return err
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&config.socketName, "target", "", "Unix domain socket name")
	viper.SetEnvPrefix("lvmetrics")
	viper.BindEnv("nodename")
	config.nodename = viper.GetString("nodename")
}
