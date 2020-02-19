apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller
  namespace: kube-system
---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tiller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller
    namespace: kube-system
---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller
  namespace: kube-system
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: helm
        name: tiller
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      automountServiceAccountToken: true
      containers:
      - env:
        - name: TILLER_NAMESPACE
          value: kube-system
        - name: TILLER_HISTORY_MAX
          value: "100"
        image: _HARBOR_IMAGE_ADDR_/helm.tiller:v2.13.1
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /liveness
            port: 44135
          initialDelaySeconds: 1
          timeoutSeconds: 1
        name: tiller
        ports:
        - containerPort: 44134
          name: tiller
        - containerPort: 44135
          name: http
        readinessProbe:
          httpGet:
            path: /readiness
            port: 44135
          initialDelaySeconds: 1
          timeoutSeconds: 1
        resources: {}
      serviceAccountName: tiller
---

apiVersion: v1
kind: Service
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller
  namespace: kube-system
spec:
  selector:
    app: helm
    name: tiller
  type: ClusterIP
  ports:
  - name: tiller
    port: 44134
    targetPort: tiller
