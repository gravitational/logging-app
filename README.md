# Gravity Logging

This gravity app provides an rsyslog-based log collection system to gravity sites.

## Overview

There are 2 main components in the logging system: collectors and forwarders.

### Forwarder

The forwarder's role is to read files on disk on each k8s node and forward them
to a central place. We use [remote_syslog2](https://github.com/papertrail/remote_syslog2)
to accomplish this, with the following [config](images/forwarder/remote_syslog.yml).
The forwarder is implemented as a `DaemonSet`.

The gist of the forwarders config is to tail all files that from `/var/log/gravity`
and `/var/log/containers`. Both these paths are mounted from the host the forwarder
is running on. Kubernetes logs are automatically included via `/var/log/containers`.

To have your applications logs forwarded from file, you must mount and log to
`/var/log/gravity`.

### Collector

The collector's role is to collect all incoming logs via an rsyslog server accepting
both TCP and UDP. The forwarder automatically sends all logs to the service provided
by the collector.

To have your application logs directly sent to rsyslog, you must log via rsyslog
TCP/UDP to `log-collector.kube-system.svc.cluster.local`.

Logs are currently written to `/var/log/messages` on the host node.

## Additional configuration

The collector mounts a `ConfigMap` named `extra-log-collector-config`. You can
write your own rsyslog configuration there to be included in the collector. This
enables you to further forward logs out of the system, to anything that speaks
incoming rsyslog.

## Building

Requirements:

* docker
* existing gravity account

To make a gravity package simply type:

```shell
make
```

## Production

This app is automatically included with any `k8s-*` app and anything inheriting from it.

## Future work

 - [ ] We pass along structured logging information transparently currently, but could do more with it.
 - [ ] Rsyslog is a pretty cranky protocol, we probably want to get rid of it eventually.
 - [ ] Consider replacing the collector with ES/Graylog, for something more indexable/searchable/extensible.

