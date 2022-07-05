# MIGRATION to topolvm.io

TODO

## Test

1. `git checkout 7b0c59f2944dc5343aa0e71cb226db53ad2f96ea`
2. `cd example && make run BUILD_IMAGE=true`
3. create some topolvm PVCs
4. `git checkout rename-group`
5. deploy new topolvm apps(`helm install`)
6. force update topolvm StorageClasses
7. check topolvm PVCs and Pods
8. create some new topolvm PVCs

## マイグレーション手順

1. TopoLVMのPVCへの操作を停止してもらう
1. 既存のTopoLVMのpodを停止する
1. KubeSchedulerConfigurationを手動でアップデートする
1. StorageClassを作り直す
1. migratorを含めた新たなpodを起動する
1. migrate結果を確認する(node/pvc/logicalvolume)
1. TopoLVMのPVCへの操作を再開する


## FAQ

1. podがスケジュールできない

PodのCapacityKeyPrefixが古い可能性あり
Podを再作成すればOKと思われる
