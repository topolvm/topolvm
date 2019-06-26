#!/bin/sh

export CA_PEM=$(cat certs/ca.pem | base64 -w 0)
cat hook-secret.yml.template | sed -e "s|{{CA_BUNDLE}}|${CA_PEM}|g" > hook-secret.yml
