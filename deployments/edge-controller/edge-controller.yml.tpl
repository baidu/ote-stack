apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-controller
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: edge-controller
      release: edge-controller
  replicas: 1
  template:
    metadata:
      labels:
        app: edge-controller
        release: edge-controller
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: edge-controller
        image: _HARBOR_IMAGE_ADDR_/edge-controller:1.0
        imagePullPolicy: IfNotPresent
        args:
        - "--kube-config=/kube/kubeconfig"
        - "--logtostderr=true"
        - "--v=2"
        volumeMounts:
        - name: kube-config
          mountPath: /kube
          readOnly: true
      volumes:
      - name: kube-config
        secret:
          secretName: kubeconfig
