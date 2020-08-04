#!/bin/sh
set -e

echo "Assuming changeset from the environment: $RIG_CHANGESET"
# note that rig does not take explicit changeset ID
# taking it from the environment variables
if [ $1 = "bootstrap" ]; then
    echo "Creating Log Forwarder ConfigMap"
    kubectl apply -f /var/lib/gravity/resources/logforwarder.yaml

    echo "Creating Log Forwarder related resources"
    kubectl apply -f /var/lib/gravity/resources/resources.yaml
elif [ $1 = "update" ]; then
    echo "Checking: $RIG_CHANGESET"
    if rig status $RIG_CHANGESET --retry-attempts=1 --retry-period=1s; then exit 0; fi

    echo "Starting update, changeset: $RIG_CHANGESET"
    rig cs delete --force -c cs/$RIG_CHANGESET
    echo "Deleting old deployments/daemonsets"
    rig delete deployments/log-collector --resource-namespace=kube-system --force
    rig delete daemonsets/log-forwarder --resource-namespace=kube-system --force
    echo "Creating or updating resources"
    rig upsert -f /var/lib/gravity/resources/resources.yaml --debug
    echo "Checking status"
    rig status $RIG_CHANGESET --retry-attempts=120 --retry-period=1s --debug
    echo "Freezing"
    rig freeze
if [ $1 = "uninstall" ]; then
    echo "Deleting Log Forwarder ConfigMap"
    kubectl apply -f /var/lib/gravity/resources/logforwarder.yaml

    echo "Deleting Log Forwarder related resources"
    kubectl apply -f /var/lib/gravity/resources/resources.yaml
elif [ $1 = "rollback" ]; then
    echo "Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/$RIG_CHANGESET
else
    echo "Missing argument, should be either 'bootstrap', 'uninstall', 'update' or 'rollback'"
fi
