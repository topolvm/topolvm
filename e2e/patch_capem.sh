#!/bin/sh

export CA_PEM=$(cat certs/ca.pem | base64 -w 0)
cat ../deploy/manifests/mutatingwebhooks.yaml | \
sed -e "s|image: quay.io/cybozu/topolvm:.*$|image: topolvm:dev\n          imagePullPolicy: Never|g" \
-e "s|clientConfig:|clientConfig:\n      caBundle: ${CA_PEM}|g" \
-e "/  annotations:$/d" \
-e "/    certmanager\.k8s\.io\/inject-ca-from:.*$/d" \
> mutatingwebhooks.yml
