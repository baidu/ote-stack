apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-recording-rules
  namespace: monitor
data:
  calculation.rules.yml: |
    # 本文件用于Prometheus预先定义公式的配置文件
    # Prometheus后台会根据公式预先计算好结果作为新指标保存到tsdb中 避免实时计算导致请求时间长甚至超时 一般用于请求获取图的接口httpQueryRangePrometheus
    groups:
    - name: recording_rules
      interval: 15s
      # BEST PRACTICES:  Recording rules should be of the general form level:metric:operations
      rules:
      # 机器所有gpu显存总大小
      - record: node:node_dcgm_fb_bytes:sum
        expr: (sum(dcgm_fb_free+dcgm_fb_used) by(instance))*1024*1024
      # 机器每块gpu显存总大小
      - record: node:gpu_dcgm_fb_bytes:sum
        expr: (dcgm_fb_free+dcgm_fb_used)*1024*1024
      # 机器每块磁盘总大小 直接使用默认metric即可
      # - record:
      #   expr: node_filesystem_size_bytes
      # 机器每块磁盘使用大小
      - record: node:node_filesystem_usage_bytes:sum
        expr: sum(node_filesystem_size_bytes-node_filesystem_free_bytes) by(instance,mountpoint)
      # 机器cpu利用率
      - record: node:node_cpu_usage_percentage:avg_rate2m
        expr: (1-avg(rate(node_cpu_seconds_total{mode="idle"}[2m])) by(instance))*100
      # 机器内存使用率
      - record: node:node_memory_usage_percentage:sum
        expr: (1-(node_memory_MemFree_bytes+node_memory_Cached_bytes+node_memory_Buffers_bytes)/node_memory_MemTotal_bytes)*100
      # 机器每块gpu使用率 直接使用默认metric即可
      # - record:
      #   expr: dcgm_gpu_utilization
      # 机器上行带宽
      - record: node:node_network_transmit_bytes:rate2m
        expr: rate(node_network_transmit_bytes_total[2m])
      # 机器上行带宽
      - record: node:node_network_receive_bytes:rate2m
        expr: rate(node_network_receive_bytes_total[2m])
      # namespace容器使用内存总大小
      - record: namespace:container_memory_usage_bytes:sum
        expr: sum(container_memory_usage_bytes{container_name!=""}) by(namespace)
      # namespace容器使用磁盘总大小
      - record: namespace:container_fs_usage_bytes:sum
        expr: sum(container_fs_usage_bytes{container_name!=""}) by(namespace)
      # namespace容器使用cpu核数总大小
      - record: namespace:container_cpu_usage_cores:sum_rate2m
        expr: sum(rate(container_cpu_usage_seconds_total{container_name!=""}[2m])) by(namespace)
      # namespace容器使用上行带宽
      - record: namespace:container_network_transmit_bytes:sum_rate2m
        expr: sum(container_network_transmit_bytes_total{container_name!=""}) by(namespace)
      # namespace容器使用下行带宽
      - record: namespace:container_network_receive_bytes:sum_rate2m
        expr: sum(container_network_receive_bytes_total{container_name!=""}) by(namespace)
      # namespace容器使用gpu显存总大小
      - record: namespace:dcgm_fb_used_bytes:sum
        expr: (sum(dcgm_fb_used{pod_name!=""}) by(pod_namespace))*1024*1024
      # pod容器使用内存总大小
      - record: pod:container_memory_usage_bytes:sum
        expr: sum(container_memory_usage_bytes{container_name!=""}) by(namespace,pod_name)
      # pod容器使用磁盘总大小
      - record: pod:container_fs_usage_bytes:sum
        expr: sum(container_fs_usage_bytes{container_name!=""}) by(namespace,pod_name)
      # pod容器使用cpu核数总大小
      - record: pod:container_cpu_usage_cores:sum_rate2m
        expr: sum(rate(container_cpu_usage_seconds_total{container_name!=""}[2m])) by(namespace,pod_name)
      # pod容器使用上行带宽
      - record: pod:container_network_transmit_bytes:sum_rate2m
        expr: sum(container_network_transmit_bytes_total{container_name!=""}) by(namespace,pod_name)
      # pod容器使用下行带宽
      - record: pod:container_network_receive_bytes:sum_rate2m
        expr: sum(container_network_receive_bytes_total{container_name!=""}) by(namespace,pod_name)
      # pod每块gpu显存总大小
      - record: pod:gpu_dcgm_fb_bytes:sum
        expr: (dcgm_fb_free+dcgm_fb_used)*1024*1024
      #  expr: dcgm_gpu_utilization
      # pod容器每块gpu使用率 直接使用默认metric即可
      # - record:
      #  expr: dcgm_gpu_utilization