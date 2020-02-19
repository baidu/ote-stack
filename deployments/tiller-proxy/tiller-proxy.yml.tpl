apiVersion: apps/v1
kind: Deployment
metadata:
  name: tiller-proxy
  namespace: kube-system
  labels:
    app: tiller-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tiller-proxy
  template:
    metadata:
      labels:
        app: tiller-proxy
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: tiller-proxy
        image: _HARBOR_IMAGE_ADDR_/tiller-proxy:0.11.1
        imagePullPolicy: IfNotPresent
        args:
        - run
        - --v=3
        - --connector=direct
        - --tiller-insecure-skip-verify=true
        - --enable-analytics=true
        - --tiller-endpoint=tiller:44134
        ports:
        - containerPort: 9855
        volumeMounts:
        - mountPath: /tmp
          name: chart-volume
      volumes:
      - name: chart-volume
        emptyDir: {}
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
---

apiVersion: v1
kind: Service
metadata:
  name: tiller-proxy
  namespace: kube-system
  labels:
    app: tiller-proxy
spec:
  ports:
  - name: http
    port: 80
    targetPort: 9855
  selector:
    app: tiller-proxy