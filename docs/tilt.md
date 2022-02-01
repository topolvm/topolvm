# Using Tilt for developing TopoLVM

[Tilt](https://tilt.dev/) is a tool to help with development of microservices.
It does it by automating rebuilding the relevant binaries and (re)deploying the components that changed.

## Deploy TopoLVM with Tilt

The e2e environment is used for tilt, so it must be set up first:

```bash
make -C e2e setup
make -C e2e start-lvmd
make -C e2e launch-kind
```

Now you can run tilt and start coding:

```bash
tilt up
```

Tilt will deploy topolvm in the KinD cluster and automatically recompile and deploy any changes you make.

## Limitations

Tilt does not handle `lvmd`.
This is beacuse `lvmd` needs root privileges and tilt cannot ask for sudo password.
It may be possible to run tilt as root, but this is not recommended.

If you make changes to `lvmd` you will instead have to compile the code manually and restart the systemd units.
