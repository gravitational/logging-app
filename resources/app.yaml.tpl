apiVersion: v1
kind: SystemApplication
metadata:
  namespace: kube-system
  repository: gravitational.io
  name: logging-app
  resourceVersion: VERSION
hooks:
  install:
    spec:
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: logging-app-bootstrap
      spec:
        template:
          metadata:
            name: logging-app-bootstrap
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:0.0.1
                command: ["/usr/local/bin/kubectl", "apply", "-f", "/var/lib/gravity/resources/resources.yaml"]

  uninstall:
    spec:
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: logging-app-uninstall
      spec:
        template:
          metadata:
            name: logging-app-uninstall
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:0.0.1
                command: ["/usr/local/bin/kubectl", "delete", "-f", "/var/lib/gravity/resources/resources.yaml"]
