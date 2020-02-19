apiVersion: v1
kind: ConfigMap
metadata:
  name: nodes-server-config
  namespace: monitor
data:
  nodes-server-config.yaml: |
    configs:
    - name: cadvisor
      value: 30
    - name: node_exporter
      value: 30
    - name: gpu
      value: 30
    - name: network_monitor
      value: 30
    - name: test
      value: 30
---

apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nodes-server
  namespace: monitor
spec:
  selector:
    matchLabels:
      app: nodes-server
  replicas: 1
  template:
    metadata:
      labels:
        app: nodes-server
    spec:
      nodeSelector:
        monitor: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      hostNetwork: true
      containers:
      - name: nodes-server
        image: _HARBOR_IMAGE_ADDR_/nodes-server:online0.3
        imagePullPolicy: IfNotPresent
        command: ["/home/nodes-server"]
        args:
        - "--config=/home/config/nodes-server-config.yaml"
        - "--logtostderr=true"
        - "--port=8999"
        volumeMounts:
        - name: nodes-server-config
          mountPath: /home/config
        ports:
        - containerPort: 8999
      volumes:
      - name: nodes-server-config
        configMap:
          name: nodes-server-config
---

apiVersion: v1
kind: Service
metadata:
  name: nodes-server
  namespace: monitor
spec:
  selector:
    app: nodes-server
  type: ClusterIP
  ports:
  - port: 8999
    targetPort: 8999
