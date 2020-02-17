apiVersion: v1
kind: ConfigMap
metadata:
  name: open-api-config
  namespace: kube-system 
data:
  app.conf: |
    appname = open-api
    httpport = 8012
    runmode = dev
    copyrequestbody = true

    allowOrigins = http://localhost:8081;http://localhost:8080

    passwordCryptoToken = abcdefa
    aesKey = 0123456789abcdef
    cbcKey = 0123456789abcdef

    jwtRefreshedSeconds = 18000
    jwtExpiredSeconds = 3600
    jwtSigningKey = abcdefg

    superAdminName = admin
    superAdminPassword = Nh@q_MrZu^wsQw]%q_P0
    superAdminNamespace = default
    systemDeploysJson = {"kube-system":["calico","coredns","traefik"],"monitor":["node-agent","node-exporter","nodes-server"]}

    harborAddress       = _HARBOR_API_ADDR_
    harborAdminUser     = _HARBOR_ADMIN_USER_
    harborAdminPassword = _HARBOR_ADMIN_PASSWD_
    repositoryDomain    = _REPOSITORY_DOMAIN_

    queryServerAddress  = http://data-query-server:8994
    elasticSearchAddress = _ELASTICSEARCH_ADDR_

    mysqlUser = root
    mysqlPwd  = 123456
    mysqlHost = ote-mysql
    mysqlDb   = sys_ote_manage_platform 
    mysqlPort = 8306

    # haloAddress = https://192.168.137.71:8989 
    # haloCa = /openapi/ssl/halo/ca.pem
    # haloCert = /openapi/ssl/halo/halo.pem
    # haloKey = /openapi/ssl/halo/halo-key.pem

    kubeConf = /openapi/ssl/kube/config

    deployStatusIntervalSeconds = 30
    deployStatusExpiredHours = 72
    alertIntervalSeconds = 600
    alertExpiredHours = 168
    alertFiringSeconds = 3600
---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: open-api
  namespace: kube-system
  labels:
    app: open-api
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: open-api
    spec:
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      initContainers:
      - name: init-mysql
        image: _HARBOR_IMAGE_ADDR_/busyboxplus:latest
        command: ['sh', '-c', 'until curl ote-mysql:8306; do echo waiting for mysql; sleep 2; done;']
      containers:
      - name: open-api
        image: _HARBOR_IMAGE_ADDR_/open-api-go:0.1.5.8
        imagePullPolicy: IfNotPresent
        volumeMounts:
        - name: halo-ssl
          mountPath: /openapi/ssl/halo
          readOnly: true
        - name: root-kube-config
          mountPath: /openapi/ssl/kube
          readOnly: true
        - name: open-api-config
          mountPath: /openapi/conf
        ports:
        - containerPort: 8012
          protocol: TCP
      volumes:
      - name: halo-ssl
        hostPath:
          path: /home/work/ote/node/halo/ssl
      - name: root-kube-config
        configMap:
          name: root-kube-config
      - name: open-api-config
        configMap:
          defaultMode: 0555
          name: open-api-config
---

apiVersion: v1
kind: Service
metadata:
  labels:
    app: open-api
  name: open-api
  namespace: kube-system
spec:
  ports:
  - name: http
    port: 8012
    protocol: TCP
    targetPort: 8012
  type: ClusterIP
  clusterIP: 10.240.0.10
  selector:
    app: open-api
