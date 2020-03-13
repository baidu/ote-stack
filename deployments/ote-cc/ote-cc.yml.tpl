apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: ote-cc
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ote-cc
      release: ote-cc
  replicas: 1
  template:
    metadata:
      labels:
        app: ote-cc
        release: ote-cc
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: ote-cc
        image: _HARBOR_IMAGE_ADDR_/ote-cc:2.1
        imagePullPolicy: IfNotPresent
        args:
        - "--kube-config=/kube/config"
        - "--remote-shim-endpoint=ote-k8s-shim:8262"
        - "--tunnel-listen=0.0.0.0:8287"
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
---

apiVersion: v1
kind: Service
metadata:
  name: ote-cc
  namespace: kube-system
spec:
  selector:
    app: ote-cc
    release: ote-cc
  type: ClusterIP
  ports:
  - port: 8287
    targetPort: 8287
