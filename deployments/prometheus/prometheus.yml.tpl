apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitor
data:
  prometheus.yml: |
    global:
      scrape_interval: 30s
      scrape_timeout: 10s
      evaluation_interval: 15s

    alerting:
      alertmanagers:
      - static_configs:
        - targets: 
          - alertmanager:8997

    rule_files:
    - "/etc/prometheus/recording-rules/*.yml"
    - "/etc/prometheus/alerting-rules/*.yml"

    scrape_configs:
    - job_name: http_cadvisor
      honor_labels: true
      metrics_path: /get_data
      params:
        name: ['cadvisor']
      static_configs:
      - targets:
        - nodes-server:8999

    - job_name: http_node_exporter
      honor_labels: true
      metrics_path: /get_data
      params:
        name: ['node_exporter']
      static_configs:
      - targets:
        - nodes-server:8999

    - job_name: http_network_monitor
      honor_labels: true
      metrics_path: /get_data
      params:
        name: ['network_monitor']
      static_configs:
      - targets:
        - nodes-server:8999

    - job_name: http_gpu
      honor_labels: true
      metrics_path: /get_data
      params:
        name: ['gpu']
      static_configs:
      - targets:
        - nodes-server:8999
---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: monitor
spec:
  selector:
    matchLabels:
      app: prometheus
  replicas: 1
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      nodeSelector:
        monitor: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      containers:
      - name: prometheus
        image: _HARBOR_IMAGE_ADDR_/prometheus:v2.15.0
        imagePullPolicy: IfNotPresent
        command: ["/bin/prometheus"]
        args:
        - "--config.file=/etc/prometheus/config/prometheus.yml"
        - "--storage.tsdb.path=/prometheus-data/"
        - "--storage.tsdb.retention.time=30d"
        - "--storage.tsdb.retention.size=50GB"
        - "--web.console.libraries=/etc/prometheus/console_libraries/"
        - "--web.console.templates=/etc/prometheus/consoles/"
        - "--web.listen-address=0.0.0.0:8998"
        ports:
        - containerPort: 8998
        securityContext:
          runAsUser: 0
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "chmod -R 777 /prometheus-data/"]
        volumeMounts:
        - name: prometheus-config
          mountPath: /etc/prometheus/config/
        - name: prometheus-recording-rules
          mountPath: /etc/prometheus/recording-rules/
        - name: prometheus-alerting-rules
          mountPath: /etc/prometheus/alerting-rules/
        - name: prometheus-data
          mountPath: /prometheus-data/
      volumes:
      - name: prometheus-config
        configMap:
          name: prometheus-config
      - name: prometheus-recording-rules
        configMap:
          name: prometheus-recording-rules
      - name: prometheus-alerting-rules
        configMap:
          name: prometheus-alerting-rules
      - name: prometheus-data
        hostPath:
          path: /home/work/ote/prometheus-data/
---

apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: monitor
spec:
  selector:
    app: prometheus
  type: ClusterIP
  ports:
  - port: 8998
    targetPort: 8998
