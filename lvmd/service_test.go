package lvmd

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc"
)

func TestService(t *testing.T) {

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	socketName := filepath.Join(dir, "lvmd.sock")
	vgName := "test"

	lis, err := net.Listen("unix", socketName)
	if err != nil {
		t.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterLVServiceServer(grpcServer, NewLVService(vgName))
	proto.RegisterVGServiceServer(grpcServer, NewVGService(vgName))
	go func() {
		grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	dialer := func(ctx context.Context, a string) (net.Conn, error) {
		return net.Dial("unix", a)
	}
	conn, err := grpc.Dial(socketName, grpc.WithInsecure(), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	c := proto.NewLVServiceClient(conn)

	resp, err := c.CreateLV(context.Background(), &proto.CreateLVRequest{Name: "test", SizeGb: 100})
	if err != nil {
		t.Fatal(err)
	}

	if resp.CommandOutput != "" {
		t.Fatal(resp.CommandOutput)
	}
}
