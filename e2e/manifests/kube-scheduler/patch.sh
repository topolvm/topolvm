#!/bin/sh -eux

current_dir=$(dirname $(readlink -f $0))

tmpdir=$(mktemp -d)
cp -r $current_dir $tmpdir
cp /etc/kubernetes/manifests/kube-scheduler.yaml $tmpdir/kube-scheduler
$current_dir/../../bin/kustomize build $tmpdir/kube-scheduler > /etc/kubernetes/manifests/kube-scheduler.yaml
