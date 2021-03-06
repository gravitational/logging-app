#!/bin/sh
set -xe

# note that rig does not take explicit changeset ID
# taking it from the environment variables

echo "--> Assuming changeset from the environment: $RIG_CHANGESET"
if [ $1 = "install" ]; then
    echo "--> Creating Log Forwarder ConfigMap"
    # apply is used here because log-forwarder could be created by gravity
    kubectl apply -f /var/lib/gravity/resources/logforwarder.yaml

    echo "--> Creating new Log Forwarder related resources"
    kubectl create -f /var/lib/gravity/resources/app

    echo "--> Waiting resources availability"
    for deployment in log-collector lr-forwarder lr-aggregator; do
	kubectl rollout status deployment $deployment
    done
    kubectl rollout status daemonset lr-collector
elif [ $1 = "update" ]; then
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
elif [ $1 = "uninstall" ]; then
    echo "--> Deleting Log Forwarder ConfigMap"
    kubectl delete -f /var/lib/gravity/resources/logforwarder.yaml

    echo "--> Deleting Log Forwarder related resources"
    for file in /var/lib/gravity/resources/app/*.yaml; do
        kubectl delete -f $file
    done
elif [ $1 = "rollback" ]; then
    echo "--> Reverting changeset $RIG_CHANGESET"
    rig revert
    rig cs delete --force -c cs/${RIG_CHANGESET}
else
    echo "--> Missing argument, should be either 'install', 'uninstall', 'update' or 'rollback'"
fi
