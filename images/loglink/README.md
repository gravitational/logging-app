## A side-car container for custom application logs

Currently, logging in kubernetes works solely on top of docker-provided logging facilities.
Docker supports multiple logging drivers but the most commonly used driver is a JSON-based encoder that captures
stdout/stderr as JSON-encoded blobs and writes them to files named using container ID.
As docker log files do not contain any kubernetes-specific metadata, in order to support pod-level log message filtering,
kubelet encodes metadata into log file names by symlinking the original docker log files into separate location.
It uses the following naming scheme:

  ```
  <pod-name>_<pod_namespace>_<container-name>-<docker-ID>.log
  ```
The biggest drawback is that it only works with docker logging. If an application is logging into a file - there is no
builtin solution available.
Following are possible options to enable the capture of file-based logs:

  1. Have application log to stdout/stderr instead (not always possible as it requires support from the application)
  1. Symlink log files as `/dev/stdout` (not always possible, especially when log files are symlinks themselves)
  1. Adapt the application container to have a `tail` process in the background transparently propagating log files to stdout/stderr (maybe considered too invasive, does not maintain proper separation of concerns available when logging into multiple files)
  1. Have a helper container take care of routing the log files in the required form to `log-forwarder`

`log-link` container implements the last option - it is a side-car container that can be hooked up to any application
to route logs to `log-forwarder` in the form that makes them available for searching and filtering.

It works by sweeping the configured directories for existing log files, symlinking them in the output directory
(the directory from which `log-forwarder` pod will consume them) employing the same naming scheme as the kubelet.

```yaml
containers:
- name: log-linker
  image: log-linker:0.0.7
  imagePullPolicy: Always
  args:
    # /loglink
    - -target-dir=/var/log/gravity
    - -watch-dir=/var/log/gravity/app-directory
  volumeMounts:
    - name: log
      mountPath: /var/log/gravity/app-directory
    - name: output
      mountPath: /var/log/gravity
  env:
    - name: POD_NAME
      valueFrom:
        fieldRef:
          fieldPath: metadata.name
    - name: POD_NAMESPACE
      valueFrom:
        fieldRef:
          fieldPath: metadata.namespace
    - name: CONTAINER_NAME
      value: app-name
volumes:
- name: log
  hostPath:
    path: /var/log/gravity/app-directory
- name: output
  hostPath:
    path: /var/log/gravity
```

Note the importance of configuring the minimal kubernetes metadata (pod name/namespace/container name tuple) for this to work.

The suggested directory structure is:
```
/var/lib/gravity
├── pod1_namespace1_container-logfile1.log
├── pod1_namespace1_container-logfile2.log
└── app-directory
      ├── logfile1.log
      └── logfile2.log

```
with `/var/lib/gravity` serving as a collector for logs where they will be picked up by `log-forwarder`.
Both `logfile1.log` and `logfile2.log` are symlinked as `pod1_namespace1_container-logfile1.log` and
`pod1_namespace1_container-logfile2.log` assuming the application runs in a pod `pod1` in the 
namespace `namspace1` in container `container`.

