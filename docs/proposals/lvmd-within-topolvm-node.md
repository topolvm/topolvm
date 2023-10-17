# Starting LVMD from within topolvm-node instead of using gRPC

## Motivation

To run TopoLVM on Edge currently requires quite a lot of memory for a minimal installation. One of the main
reasons for that is due to the lvmd container running as a separate DaemonSet from the topolvm-node DaemonSet.
The idea is to combine these two into a single DaemonSet to reduce the memory footprint as they are usually run
within the same pod anyway when lvmd is not running as a systemd service.

### Goals

- Unify the topolvm-node and lvmd containers into a single container and not have to communicate via gRPC.
- Allow lvmd to run as a systemd service or as a DaemonSet like before.
- Do not break existing installations.
- Reduce memory footprint.

## Proposal

TopoLVM is a storage plugin based on [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

To manage LVM, `lvmd` should be run as a system service of the node OS.
It provides gRPC services via UNIX domain socket to create/update/delete
LVM logical volumes and watch a volume group status.

`topolvm-node` implements CSI node services as well as miscellaneous control
on each Node.  It communicates with `lvmd` to watch changes in free space
of a volume group and exports the information by annotating Kubernetes
`Node` resource of the running node.  In the meantime, it adds a finalizer
to the `Node` to clean up PersistentVolumeClaims (PVC) bound on the node.

Since both topolvm-node and lvmd need to run as DaemonSet, we can simply combine them and start them under a single binary.

## Design Details

### Allowing topolvm-node to start lvmd

The main problem with running lvmd as a systemd service or DaemonSet is that it is not possible to start it from within the topolvm-node container.
We can fix this by allowing topolvm-node to start lvmd by compiling a service into its startup command.
This can be enabled via flag and then use a wrapper to call lvmd server commands directly from the client.

### Flow of Operations for lvmd running within topolvm-node

1. topolvm-node starts and checks if lvmd should be started as well via `--embed-lvmd` flag.
2. If `--embed-lvmd` is set, it will not initialize gRPC clients via Connection but call a dedicated wrapper instead.
3. The wrapper generates a `proto.LVServiceClient` and `proto.VGServiceClient` by calling `lvmd.NewEmbeddedServiceClients`.
   The `lvmd.NewEmbeddedServiceClients` function will create the Services and bind the client directly to them:
   ```go
   // NewEmbeddedServiceClients creates clients locally calling instead of using gRPC.
   func NewEmbeddedServiceClients(ctx context.Context, dcmapper *DeviceClassManager, ocmapper *LvcreateOptionClassManager) (
       proto.LVServiceClient,
       proto.VGServiceClient,
   ) {
       vgServiceServerInstance, notifier := NewVGService(dcmapper)
       lvServiceServerInstance := NewLVService(dcmapper, ocmapper, notifier)
	   // use the local wrapper to redirect client calls to the service instances.
       caller := &localCaller{
           lvServiceServer: lvServiceServerInstance,
           vgServiceServer: vgServiceServerInstance.(*vgService),
       }
	   // automatically start a server watch so that watch clients get notified until context is cancelled.
       caller.vgWatch = &localWatch{ctx: ctx, watch: make(chan any)}
       go caller.vgServiceServer.Watch(&proto.Empty{}, caller.vgWatch)
       return caller, caller
   }
   ```
4. Now whenever a returned client from this function is used, instead of using gRPC, the code is called directly:
   ```go
   // *localCaller is the reference to the wrapper containing the generated server.
   // It also implements the proto.LVServiceClient and proto.VGServiceClient interfaces and just calls the server functions.
   func (l *localCaller) CreateLV(ctx context.Context, in *proto.CreateLVRequest, _ ...grpc.CallOption) (*proto.CreateLVResponse, error) {
        return l.lvServiceServer.CreateLV(ctx, in)
   }
   ```
   For `vGServiceWatchClient` and `vGServiceWatchServer` an unbounded blocking channel is used to simulate the gRPC stream.
5. The returned `proto.LVServiceClient` and `proto.VGServiceClient` are used to create the `LogicalVolumeReconciler`.
6. Also, the health checker and metrics exporter are created with the wrapped clients.
7. `driver.NewNodeServer` is created with the wrapper and started. Any call that would be made via gRPC for lvm interaction is now done locally.
8. Now we can completely remove the lvmd container from the rendered manifest and only start topolvm-node. We also no longer need
   to supply it with a gRPC socket.

As one can see, this is a simple wrapper around the lvmd server that is used to call lvmd code behind the usual grpc server from within the topolvm-node service.

We can easily integrate it on `pkg/topolvm-node/cmd` by introducing the following flags:

```go
func init() {
	//...
	fs.BoolVar(&config.embedLvmd, "embed-lvmd", false, "Runs LVMD embedded within topolvm-node")
	fs.StringVar(&cfgFilePath, "config", filepath.Join("/etc", "topolvm", "lvmd.yaml"), "config file")
    //...

	fs.AddGoFlagSet(goflags)
}
```

After this, we can use the flags taken from the lvmd binary and start it within the command.
We will default the `command.Containerized` flag as an embedded version of lvmd will always be containerized in the deployment.
We can also default the lvmd logging library to JSON logging.

## Caveats

- LVMD can no longer be deployed and managed separately from topolvm-node if started together.
- We will need to add HELM Chart configurations for this style of deployment.
- We will need to add matrix tests like for systemd-lvmd and DaemonSet-lvmd to ensure full QA capabilities.

## Packaging and deployment

`lvmd` is no longer necessary as a binary and can completely ommitted in a deployment without lvmd as a systemd service.
All other components are unaffected, but topolvm-node needs to be started correctly if this should be exposed in the helm chart.
