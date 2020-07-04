apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: ote-k8s-shim
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: ote-k8s-shim
      release: ote-k8s-shim
  replicas: 1
  template:
    metadata:
      labels:
        app: ote-k8s-shim
        release: ote-k8s-shim
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: ote-k8s-shim
        image: _HARBOR_IMAGE_ADDR_/ote-shim:3.0
        imagePullPolicy: IfNotPresent
        args:
        - "--kube-config=/kube/config"
        - "--helm-addr=tiller-proxy:80"
        - "--listen=0.0.0.0:8262"
        - "--logtostderr=true"
        - "--v=2"
        volumeMounts:
        - name: edge-kube-config
          mountPath: /kube/config
          subPath: config
        ports:
        - containerPort: 8262
      volumes:
      - name: edge-kube-config
        configMap:
          name: edge-kube-config
---

apiVersion: v1
kind: Service
metadata:
  name: ote-k8s-shim
  namespace: kube-system
spec:
  selector:
    app: ote-k8s-shim
    release: ote-k8s-shim
  type: ClusterIP
  ports:
  - port: 8262
    targetPort: 8262

