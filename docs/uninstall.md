# Uninstall

To uninstall TopoLVM, please follow these steps.

> [!CAUTION]
> The uninstall procedure cannot be reverted. Think twice whether you can really uninstall TopoLVM.

1. Delete all the snapshots and PVCs related to TopoLVM.
2. If the scheduler extender is enabled, edit the kube-scheduler's config file and remove TopoLVM's settings. It may be your help to read [the configuration guide](advanced-setup##using-topolvm-scheduler).
   - Please be careful if you have any other settings in extenders section of the configuration file. You can remove only the TopoLVM's setting.
3. Uninstall TopoLVM via Helm.
   ```bash
   # Check the namespace
   helm list -A
   
   # Uninstall. Here we assume the namespace is `topolvm-system`.
   helm uninstall --namespace=topolvm-system topolvm
   ```
4. Remove a label from the target namespace.  
   - When legacy mode is disabled:
     ```bash
     # Please change the namespace depending on your environment.
     kubectl label namespace topolvm-system topolvm.io/webhook-
     kubectl label namespace kube-system topolvm.io/webhook-
     ```
   - When legacy mode is enabled:
     ```bash
     # Please change the namespace depending on your environment.
     kubectl label namespace topolvm-system topolvm.cybozu.com/webhook-
     kubectl label namespace kube-system topolvm.cybozu.com/webhook-
     ```
5. If LVMd is running as a systemd service, stop it on each node.
   ```bash
   systemctl is-active lvmd.service
   systemctl disable --now lvmd.service
   ```
