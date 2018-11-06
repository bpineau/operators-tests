#!/bin/bash

[ -z "$DATADOG_API_KEY" ] && \
  echo "export DATADOG_API_KEY, DATADOG_APP_KEY, DATADOG_HOST before launching this" && exit 1

# purge old installation
kubectl delete compositecontroller mon-controller
kubectl delete crd mon.ddhq.com
kubectl delete namespace tmon
until ! kubectl get ns tmon >/dev/null 2>&1; do sleep 1 ; done

kubectl create namespace tmon

# install metacontroller himself
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

# metacontroller don't release often, and we need recent support for finalizers
kubectl -n metacontroller set image pod metacontroller-0 metacontroller=metacontroller/metacontroller:latest

# this secret will be mounted in the "webhook" (webhook is a simple python deployment)
kubectl -n tmon create secret generic dd-api-secret \
  --from-literal=DATADOG_API_KEY=$DATADOG_API_KEY \
  --from-literal=DATADOG_APP_KEY=$DATADOG_APP_KEY \
  --from-literal=DATADOG_HOST=$DATADOG_HOST

# deploy the python code in a configmap that will be mounted by the "webhook"
kubectl -n tmon create configmap mon-controller --from-file=sync.py

# create the "webhook": a python webserver deployment, and the service exposing him to metacontroller
kubectl apply -f webhook.yaml

# this compositecontroller registers the webhook's service for all the "mons.ddhq.com/v1" CRD events
kubectl apply -f compositecontroller.yaml

# declare the mons.ddhq.com/v1 CRD
kubectl apply -f crd.yaml

# plus qu'a creer des monitors avec par ex.:
# kubectl apply -f test-monitor.yaml
