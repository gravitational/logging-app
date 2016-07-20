apiVersion: v1
kind: Service
metadata:
  name: log-collector
  namespace: kube-system
spec:
  type: ClusterIP
  ports:
    - name: rsyslog-udp
      protocol: UDP
      port: 514
      targetPort: 5514
    - name: rsyslog-tcp
      protocol: TCP
      port: 514
      targetPort: 5514
  selector:
    role: log-collector
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: extra-log-collector-config
  namespace: kube-system
---
apiVersion: v1
kind: ReplicationController
metadata:
  labels:
    role: log-collector
    name: log-collector
    version: v1
  name: log-collector
  namespace: kube-system
spec:
  replicas: 1
  selector:
    role: log-collector
    version: v1
  template:
    metadata:
      labels:
        role: log-collector
        version: v1
    spec:
      containers:
        - name: log-collector
          image: log-collector:VERSION
          imagePullPolicy: Always
          ports:
            - name: udp
              protocol: UDP
              containerPort: 514
            - name: tcp
              protocol: TCP
              containerPort: 514
          volumeMounts:
            - name: varlog
              mountPath: /var/log
            - name: extra-log-collector-config
              mountPath: /etc/rsyslog.d
      volumes:
        - name: extra-log-collector-config
          configMap:
            name: extra-log-collector-config
        - name: varlog
          hostPath:
            path: /var/log

---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: log-forwarder
  namespace: kube-system
spec:
  template:
    metadata:
      labels:
        name: log-forwarder
    spec:
      containers:
        - name: log-forwarder
          image: log-forwarder:VERSION
          imagePullPolicy: Always
          volumeMounts:
            - name: gravitylog
              mountPath: /var/log/gravity
            - name: varlogcontainers
              mountPath: /var/log/containers
            - name: extdockercontainers
              mountPath: /ext/docker/containers
      volumes:
        - name: gravitylog
          hostPath:
            path: /var/log/gravity
        - name: varlogcontainers
          hostPath:
            path: /var/log/containers
        - name: extdockercontainers
          hostPath:
            path: /ext/docker/containers

