apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: monitor
data:
  alertmanager.yml: |
    global:
      resolve_timeout: 5m
      smtp_smarthost: 127.0.0.1:25  # 邮箱smtp服务器代理
      smtp_from: v2x@ote  #邮件接收人员
      smtp_require_tls: false

    route:
      receiver: 'default_receiver'   # 配置默认路由策略
      group_wait: 30s     # 最初告警通知等待时间
      group_by: ['alertname']
      group_interval: 5m  # 告警新通知等待时间
      repeat_interval: 1h # 重复告警等待时间

    receivers:
    - name: 'default_receiver'   # 与如上的default_reveiver匹配
      # email_configs:    # 邮件告警配置
      # - to: v2xalert-ote@baidu.com
      #   headers: { Subject: "[WARN] K8S报警邮件"}
      webhook_configs:   # web平台告警配置
      - url: 'http://10.240.0.10:8012/v1/alert/callback'
        http_config:
          tls_config:
            insecure_skip_verify: true

    inhibit_rules:   # 告警抑制规则
    - source_match:
        severity: 'critical'
      target_match:
        severity: 'warning'
      equal: ['alertname', 'dev', 'instance']
---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: alertmanager
  namespace: monitor
spec:
  selector:
    matchLabels:
      app: alertmanager
  replicas: 1
  template:
    metadata:
      labels:
        app: alertmanager
    spec:
      nodeSelector:
        monitor: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      containers:
      - name: alertmanager
        image: _HARBOR_IMAGE_ADDR_/alertmanager:v0.20.0
        imagePullPolicy: IfNotPresent
        command: ["/bin/alertmanager"]
        args:
        - "--config.file=/etc/alertmanager/config/alertmanager.yml"
        - "--storage.path=/alertmanager-data/"
        - "--data.retention=120h"
        - "--web.listen-address=0.0.0.0:8997"
        ports:
        - containerPort: 8997
        securityContext:
          runAsUser: 0
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "chmod -R 777 /alertmanager-data/"]
        volumeMounts:
        - name: alertmanager-config
          mountPath: /etc/alertmanager/config/
        - name: alertmanager-data
          mountPath: /alertmanager-data/
      volumes:
      - name: alertmanager-config
        configMap:
          name: alertmanager-config
      - name: alertmanager-data
        hostPath:
          path: /home/work/ote/alertmanager-data/
---

apiVersion: v1
kind: Service
metadata:
  name: alertmanager
  namespace: monitor
spec:
  selector:
    app: alertmanager
  type: ClusterIP
  ports:
  - port: 8997
    targetPort: 8997
