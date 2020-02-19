apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-apiserver-token-config
  namespace: kube-system
data:
  token.csv: |
    365719038f0aafe86eb9f8bbadf48955,kubelet-bootstrap,10001
---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: kube-apiserver
  replicas: 1
  template:
    metadata:
      labels:
        app: kube-apiserver
    spec:
      nodeSelector:
        cc: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      hostNetwork: true
      containers:
      - name: kube-apiserver
        image: _HARBOR_IMAGE_ADDR_/kube-apiserver:v1.12.5
        imagePullPolicy: IfNotPresent
        command: ["/usr/local/bin/kube-apiserver"]
        args:
        - "--etcd-servers=http://127.0.0.1:9379"
        - "--token-auth-file=/home/config/token.csv"
        - "--disable-admission-plugins=ServiceAccount"
        - "--allow-privileged=true"
        - "--service-cluster-ip-range=10.254.0.0/12"
        - "--insecure-bind-address=0.0.0.0"
        - "--insecure-port=9082"
        - "--advertise-address=_KUBE_APISERVER_HOST_IP_"
        - "--logtostderr=true"
        - "--v=2"
        volumeMounts:
        - name: kube-apiserver-token-config
          mountPath: /home/config/
        ports:
        - containerPort: 9082
      volumes:
      - name: kube-apiserver-token-config
        configMap:
          name: kube-apiserver-token-config
---

apiVersion: v1
kind: Service
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  selector:
    app: kube-apiserver
  type: ClusterIP
  ports:
  - port: 9082
    targetPort: 9082
