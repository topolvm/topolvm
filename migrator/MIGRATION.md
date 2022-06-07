# MIGRATION to topolvm.io

TODO

## Test

1. `git checkout 7b0c59f2944dc5343aa0e71cb226db53ad2f96ea`
2. `cd example && make run`
3. create some topolvm PVCs
4. `git checkout rename-group`
5. deploy new topolvm apps(`helm install`)
6. force update topolvm StorageClasses
7. check topolvm PVCs and Pods
8. create some new topolvm PVCs
