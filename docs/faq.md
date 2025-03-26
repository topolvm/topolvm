# Frequently Asked Questions <!-- omit in toc -->

<!-- Created by VSCode Markdown All in One command: Create Table of Contents -->
- [The Pod does not start when nodeName is specified in the Pod spec](#the-pod-does-not-start-when-nodename-is-specified-in-the-pod-spec)

## The Pod does not start when nodeName is specified in the Pod spec

When you specify `nodeName` in the Pod spec, the Pod may not start. If this happens, the following events are recorded in the Pod and PVC.

Pod:
```
$ kubectl describe pod <pod-name>
...
Events:
  Type    Reason                Age                   From                         Message
  ----    ------                ----                  ----                         -------
  Normal  WaitForFirstConsumer  2m40s (x82 over 22m)  persistentvolume-controller  waiting for first consumer to be created before binding
```

PVC:
```
$ kubectl describe pvc <pvc-name>
...
Events:
  Type     Reason       Age                 From     Message
  ----     ------       ----                ----     -------
  Warning  FailedMount  76s (x47 over 11m)  kubelet  Unable to attach or mount volumes: unmounted volumes=[my-volume], unattached volumes=[], failed to process volumes=[my-volume]: error processing PVC default/pvc-1: PVC is not bound
```

To avoid this, use node affinity instead of nodeName. You can get more information about this issue in the below link:
- [Support the pod used nodeName schedule, volume controller can binding pvc to pod. #93145 comment](https://github.com/kubernetes/kubernetes/issues/93145#issuecomment-661582540)
