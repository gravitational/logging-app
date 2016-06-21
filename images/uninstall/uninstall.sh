#!/bin/sh
kubectl delete \
   job/log-bootstrap \
   rc/log-collector \
   daemonset/log-forwarder \
   configmap/extra-log-collector-config \
   svc/log-collector
