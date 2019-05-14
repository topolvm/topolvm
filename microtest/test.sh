#!/bin/sh

for i in $(seq 600); do
    sleep 1
    val=$(/snap/bin/microk8s.kubectl get node -o json | jq -r '.items[0].metadata.annotations["topolvm.cybozu.com/capacity"]')
    if [ "$val" = 5368709120 ]; then
        exit 0
    fi
done
exit 1
