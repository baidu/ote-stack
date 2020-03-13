apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: node-exporter
  namespace: monitor
spec:
  template:
    metadata:
      name: node-exporter
      labels:
        app: node-exporter
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '9100'
        prometheus.io/path: 'metrics'
    spec:
      hostNetwork: true
      hostPID: true
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: node-exporter
        # use prom/node-exporter:v0.18.1 image instead of v0.18.0 while v0.18.0 is not supported on aarch64
        image: _HARBOR_IMAGE_ADDR_/node-exporter:v0.18.1
        imagePullPolicy: IfNotPresent
        args:
        - "--web.listen-address=0.0.0.0:9100"
        - "--web.disable-exporter-metrics"
        - "--path.procfs=/host/proc"
        - "--path.sysfs=/host/sys"
        - "--path.rootfs=/host/root"
        - "--collector.cpu"
        - "--collector.meminfo"
        - "--collector.loadavg"
        - "--collector.netdev"
        - "--collector.netdev.ignored-devices=cali*|natgre|veth*|lo*|virbr*|gre*"
        - "--collector.filesystem"
        - "--collector.filesystem.ignored-mount-points=dev|proc|sys|noah|matrix|var/lib*|/home/docker/data*"
        - "--collector.filesystem.ignored-fs-types=tmpfs|cgroup|overlay|rootfs"
        - "--no-collector.uname"
        - "--no-collector.netclass"
        - "--no-collector.arp"
        - "--no-collector.bcache"
        - "--no-collector.bonding"
        - "--no-collector.conntrack"
        - "--no-collector.cpufreq"
        - "--no-collector.diskstats"
        - "--no-collector.edac"
        - "--no-collector.entropy"
        - "--no-collector.filefd"
        - "--no-collector.hwmon"
        - "--no-collector.infiniband"
        - "--no-collector.ipvs"
        - "--no-collector.mdadm"
        - "--no-collector.netstat"
        - "--no-collector.nfs"
        - "--no-collector.nfsd"
        - "--no-collector.pressure"
        - "--no-collector.sockstat"
        - "--no-collector.stat"
        - "--no-collector.time"
        - "--no-collector.timex"
        - "--no-collector.vmstat"
        - "--no-collector.xfs"
        - "--no-collector.zfs"
        ports:
        - name: metrics
          containerPort: 9100
        volumeMounts:
        - name: proc
          readOnly: true
          mountPath: /host/proc
        - name: sys
          readOnly: true
          mountPath: /host/sys
        - name: root
          readOnly: true
          mountPath: /host/root
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: root
        hostPath:
          path: /
---

apiVersion: v1
kind: Service
metadata:
  name: node-exporter
  namespace: monitor
  labels:
    name: node-exporter
spec:
  selector:
    app: node-exporter
  type: ClusterIP
  ports:
  - name: node-exporter
    port: 9100
    targetPort: 9100

