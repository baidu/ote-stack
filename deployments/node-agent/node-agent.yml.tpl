apiVersion: v1
kind: ConfigMap
metadata:
  name: node-agent-config
  namespace: monitor
data:
  node-agent-config.yaml: |
    jobs:
    - name: cadvisor
      localAddr: http://127.0.0.1:6028/metrics/cadvisor
      remoteAddr: _NODES_SERVER_ADDR_/post_data
      scrapeInterval: 30
      filter:
      - container_cpu_usage_seconds_total
      - container_memory_usage_bytes
      - container_network_transmit_bytes_total
      - container_network_receive_bytes_total
      - container_fs_usage_bytes
      - machine_cpu_cores
      - machine_memory_bytes
    - name: node_exporter
      localAddr: http://127.0.0.1:9100/metrics
      remoteAddr: _NODES_SERVER_ADDR_/post_data
      scrapeInterval: 30
      filter:
      - node_cpu_seconds_total
      - node_filesystem_size_bytes
      - node_filesystem_free_bytes
      - node_memory_MemTotal_bytes
      - node_memory_MemFree_bytes
      - node_memory_Cached_bytes
      - node_memory_Buffers_bytes
      - node_network_transmit_bytes_total
      - node_network_receive_bytes_total
      - node_load5
    - name: gpu
      localAddr: http://127.0.0.1:9400/gpu/metrics
      remoteAddr: _NODES_SERVER_ADDR_/post_data
      scrapeInterval: 30
      filter:
      - dcgm_fb_free
      - dcgm_fb_used
      - dcgm_gpu_utilization
---

apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: node-agent
  namespace: monitor
  labels:
    app: node-agent
spec:
  template:
    metadata:
      name: node-agent
      labels:
        app: node-agent
    spec:
      hostNetwork: true
      # dnsPolicy: ClusterFirstWithHostNet
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - name: node-agent
        image: _HARBOR_IMAGE_ADDR_/node-agent:online0.1
        imagePullPolicy: IfNotPresent
        command: ["/home/node-agent"]
        args:
        - "--config=/home/config/node-agent-config.yaml"
        - "--logtostderr=true"
        - "--queue_size=100"
        - "--retry_num=3"
        - "--retry_interval=1"
        volumeMounts:
        - name: node-agent-config
          mountPath: /home/config
      volumes:
      - name: node-agent-config
        configMap:
          name: node-agent-config