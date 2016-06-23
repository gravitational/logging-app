#!/bin/sh

kubectl delete --namespace kube-system \
   job/log-bootstrap \
   rc/log-collector \
   daemonset/log-forwarder \
   configmap/extra-log-collector-config \
   svc/log-collector
