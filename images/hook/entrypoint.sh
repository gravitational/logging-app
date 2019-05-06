#!/bin/sh
set -e

# note that rig does not take explicit changeset ID
# taking it from the environment variables

echo "Assuming changeset from the environment: $RIG_CHANGESET"
if [[ $1 = "update" ]]; then
    echo "Checking: $RIG_CHANGESET"
    if rig status ${RIG_CHANGESET} --retry-attempts=1 --retry-period=1s; then exit 0; fi

    echo "Starting update, changeset: $RIG_CHANGESET"
    rig cs delete --force -c cs/${RIG_CHANGESET}

    echo "Deleting old deployments/daemonsets"
    # removing 'old' logging-app resources
    rig delete daemonsets/log-forwarder --resource-namespace=kube-system --force
    rig delete deployments/log-collector --resource-namespace=kube-system --force

    # removing 'new' logging-app resources
    rig delete daemonsets/lr-collector --resource-namespace=kube-system --force
    rig delete deployments/lr-forwarder --resource-namespace=kube-system --force
    rig delete deployments/lr-aggregator --resource-namespace=kube-system --force

    echo "Creating or updating resources"
    rig upsert -f /var/lib/gravity/resources/app/lr-aggregator.yaml --debug
    rig upsert -f /var/lib/gravity/resources/app/lr-collector.yaml --debug
    rig upsert -f /var/lib/gravity/resources/app/lr-forwarder.yaml --debug
    rig upsert -f /var/lib/gravity/resources/app/log-collector.yaml --debug

    echo "Checking status"
    rig status ${RIG_CHANGESET} --retry-attempts=120 --retry-period=1s --debug

    echo "Freezing"
    rig freeze

elif [[ $1 = "rollback" ]]; then
    echo "Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/${RIG_CHANGESET}

else
    echo "Missing argument, should be either 'update' or 'rollback'"

fi
