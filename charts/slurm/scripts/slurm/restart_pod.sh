#!/bin/bash

APISERVER=https://kubernetes.default.svc
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
POD_NAME=${K8S_POD_NAME:-$(hostname)}

curl -k \
  -X DELETE \
  -H "Authorization: Bearer $TOKEN" \
  ${APISERVER}/api/v1/namespaces/${NAMESPACE}/pods/${POD_NAME}
