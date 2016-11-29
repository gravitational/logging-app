#!/usr/bin/dumb-init /bin/sh
set -e

if [ $1 = "update" ]; then
    echo "Starting update, changeset: $RIG_CHANGESET"
    echo "Deleting old replication controller rc/log-forwarder"
    rig delete rc/log-collector --resource-namespace=kube-system
    echo "Creating or updating resources"
    rig upsert -f /var/lib/gravity/resources/resources.yaml --debug
    echo "Checking status"
    rig status $RIG_CHANGESET --retry-attempts=120 --retry-period=1s --debug
    echo "Freezing"
    rig freeze
else
    echo "Rolling back, changeset: $RIG_CHANGESET"
    rig rollback
fi
