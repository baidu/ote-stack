apiVersion: v1
kind: ConfigMap
metadata:
  name: data-query-server-config
  namespace: kube-system
data:
  config.yml: |
    configs:
    - name: prometheusAddr
      value: _PROMETHEUS_ADDR_
    - name: allCluster
      value: all|root
    - name: minStep
      value: 1m
    - name: disk
      value: /home|/tmp|/var|/home/disk.+|/ssd.+|/
---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: data-query-server
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: data-query-server
  replicas: 1
  template:
    metadata:
      labels:
        app: data-query-server
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: data-query-server
        image: _HARBOR_IMAGE_ADDR_/data-query-server:multi-cluster-2020.01.20.16.03.22
        imagePullPolicy: IfNotPresent
        volumeMounts:
        - name: data-query-server-config
          mountPath: /home/config
        ports:
        - containerPort: 8994
      volumes:
      - name: data-query-server-config
        configMap:
          name: data-query-server-config
---
 
apiVersion: v1
kind: Service
metadata:
  name: data-query-server
  namespace: kube-system
spec:
  selector:
    app: data-query-server
  type: ClusterIP
  ports:
  - port: 8994
    targetPort: 8994
