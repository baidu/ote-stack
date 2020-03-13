apiVersion: v1
kind: ConfigMap
metadata:
  name: root-kube-config
  namespace: kube-system
data:
  config: |
    apiVersion: v1
    kind: Config
    preferences: {}
    current-context: default
    clusters:
    - name: root
      cluster:
        insecure-skip-tls-verify: true
        server: http://kube-apiserver:9082
    contexts:
    - name: default
      context:
        cluster: root
        user: cluster-admin
    users:
    - name: cluster-admin
      user:
        token: 365719038f0aafe86eb9f8bbadf48955
---

apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: ote-cm
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ote-cm
      release: ote-cm
  replicas: 1
  template:
    metadata:
      labels:
        app: ote-cm
        release: ote-cm
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: ote-cm
        image: _HARBOR_IMAGE_ADDR_/ote-cm:2.1
        imagePullPolicy: IfNotPresent
        args:
        - "--kube-config=/kube/config"
        - "--root-cluster-controller=ote-cc:8287"
        - "--logtostderr=true"
        - "--v=2"
        volumeMounts:
        - name: root-kube-config
          mountPath: /kube/config
          subPath: config
        ports:
        - containerPort: 8287
      volumes:
      - name: root-kube-config
        configMap:
          name: root-kube-config

