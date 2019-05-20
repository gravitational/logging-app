#!/bin/sh
set -e

# note that rig does not take explicit changeset ID
# taking it from the environment variables

echo "--> Assuming changeset from the environment: $RIG_CHANGESET"
if [ $1 = "update" ]; then
    echo "--> Checking: $RIG_CHANGESET"
    if rig status ${RIG_CHANGESET} --retry-attempts=1 --retry-period=1s; then exit 0; fi

    echo "--> Starting update, changeset: $RIG_CHANGESET"
    rig cs delete --force -c cs/${RIG_CHANGESET}

    echo "--> Deleting old deployments"
    for deployment in log-collector lr-forwarder lr-aggregator; do
        rig delete deployments/$deployment --resource-namespace=kube-system --force
    done

    echo "--> Deleting old daemonsets"
    for daemonset in log-forwarder lr-collector; do
        rig delete daemonsets/$daemonset --resource-namespace=kube-system --force
    done

    echo "--> Creating new resources"
    for file in /var/lib/gravity/resources/app/*.yaml; do
        rig upsert -f $file --debug
    done

    echo "--> Checking status"
    rig status ${RIG_CHANGESET} --retry-attempts=120 --retry-period=1s --debug

    echo "--> Freezing"
    rig freeze

elif [ $1 = "rollback" ]; then
    echo "--> Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/${RIG_CHANGESET}

else
    echo "--> Missing argument, should be either 'update' or 'rollback'"

fi
