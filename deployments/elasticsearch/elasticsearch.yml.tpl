apiVersion: apps/v1beta2
kind: Deployment
metadata:
  name: elasticsearch
  namespace: monitor
spec:
  selector:
    matchLabels:
      app: elasticsearch
  template:
    metadata:
      labels:
        app: elasticsearch
    spec:
      nodeSelector:
        log: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      hostNetwork: true
      containers:
      - name: elasticsearch
        image: _HARBOR_IMAGE_ADDR_/elasticsearch:6.5.3
        imagePullPolicy: IfNotPresent
        # securityContext:
        #  runAsUser: 0
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "chmod -R 777 /usr/share/elasticsearch/data"]
        volumeMounts:
        - name: es-data
          mountPath: /usr/share/elasticsearch/data
        env:
        - name: discovery.type
          value: single-node
        ports:
        - containerPort: 9200
          name: http
          protocol: TCP
      volumes:
      - name: es-data
        hostPath:
          path: /home/work/ote/elasticsearch-data/
---

apiVersion: v1
kind: Service
metadata:
  name: elasticsearch
  namespace: monitor
spec:
  selector:
    app: elasticsearch
  type: ClusterIP
  ports:
  - port: 9200
    targetPort: 9200
---

apiVersion: v1
kind: ConfigMap
metadata:
  name: curator-config
  namespace: monitor
  labels:
    app: curator
data:
  action_file.yml: |-
    ---
    # Also remember that all examples have 'disable_action' set to True.  If you
    # want to use this action as a template, be sure to set this to False after
    # copying it.
    actions:
      1:
        action: delete_indices
        description: "Clean up ES by deleting old indices"
        options:
          timeout_override:
          continue_if_exception: False
          disable_action: False
          ignore_empty_list: True
        filters:
        - filtertype: pattern
          kind: prefix
          value: log-
        - filtertype: age
          source: name
          direction: older
          timestring: '%Y.%m.%d'
          unit: days
          unit_count: 2
  config.yml: |-
    ---
    client:
      hosts:
        - elasticsearch
      port: 9200
      url_prefix:
      use_ssl: False
      certificate:
      client_cert:
      client_key:
      ssl_no_validate: False
      http_auth:
      timeout: 60
      master_only: False
    logging:
      loglevel: INFO
      logfile:
      logformat: default
      blacklist: ['elasticsearch', 'urllib3']
---

apiVersion: batch/v2alpha1
kind: CronJob
metadata:
  name: curator
  namespace: monitor
  labels:
    app: curator
spec:
  schedule: "0 * * * *"
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 3
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 120
  jobTemplate:
    spec:
      template:
        spec:
          imagePullSecrets:
          - name: _HARBOR_SECRET_NAME_
          containers:
          - image: _HARBOR_IMAGE_ADDR_/curator:5.7.6
            name: curator
            args: ["--config", "/etc/config/config.yml", "/etc/config/action_file.yml"]
            volumeMounts:
            - name: curator-config
              mountPath: /etc/config
          volumes:
          - name: curator-config
            configMap:
              name: curator-config
          restartPolicy: OnFailure