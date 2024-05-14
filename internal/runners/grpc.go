package runners

import (
	"context"
	"net"
	"os"

	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type gRPCServerRunner struct {
	srv            *grpc.Server
	sockFile       string
	leaderElection bool
}

var _ manager.LeaderElectionRunnable = gRPCServerRunner{}

// NewGRPCRunner creates controller-runtime's manager.Runnable for a gRPC server.
// The server will listen on UNIX domain socket at sockFile.
// If leaderElection is true, the server will run only when it is elected as leader.
func NewGRPCRunner(srv *grpc.Server, sockFile string, leaderElection bool) manager.Runnable {
	return gRPCServerRunner{srv, sockFile, leaderElection}
}

// Start implements controller-runtime's manager.Runnable.
func (r gRPCServerRunner) Start(ctx context.Context) error {
	err := os.Remove(r.sockFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lis, err := net.Listen("unix", r.sockFile)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		r.srv.GracefulStop()
	}()

	return r.srv.Serve(lis)
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (r gRPCServerRunner) NeedLeaderElection() bool {
	return r.leaderElection
}
