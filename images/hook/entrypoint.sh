#!/bin/sh
set -e

echo "Asuming changeset from the envrionment: $RIG_CHANGESET"
# note that rig does not take explicit changeset ID
# taking it from the environment variables
if [ $1 = "update" ]; then
    echo "Starting update, changeset: $RIG_CHANGESET"
    echo "Deleting old replication controller rc/log-forwarder"
    rig delete rc/log-collector --resource-namespace=kube-system --force
    echo "Creating or updating resources"
    rig upsert -f /var/lib/gravity/resources/resources.yaml --debug
    echo "Checking status"
    rig status $RIG_CHANGESET --retry-attempts=120 --retry-period=1s --debug
    echo "Freezing"
    rig freeze
elif [ $1 = "rollback" ]; then
    echo "Rolling back, changeset: $RIG_CHANGESET"
    rig rollback
else
    echo "Missing argument, should be either 'update' or 'rollback'"
fi
