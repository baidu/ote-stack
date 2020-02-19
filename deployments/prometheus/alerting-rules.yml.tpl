apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-alerting-rules
  namespace: monitor
data:
  cpu.yml: |+
    groups:
    - name: NodeCPU
      rules:
      - alert: NodeCPUUsage
        expr: node:node_cpu_usage_percentage:avg_rate2m > 80
        for: 30s
        labels:
          user: car
          limit: 80%
          ipaddr: "{{$labels.ipaddr}}"
        annotations:
          summary: "{{$labels.instance}}: High CPU usage detected"
          description: "{{$labels.instance}}: CPU usage is above 80%"
          value: "{{$value}}"

  mem.yml: |+
    groups:
    - name: NodeCPU
      rules:
      - alert: NodeCPUUsage
        expr: node:node_memory_usage_percentage:sum > 80
        for: 30s
        labels:
          user: car
          limit: 80%
          ipaddr: "{{$labels.ipaddr}}"
        annotations:
          summary: "{{$labels.instance}}: High CPU usage detected"
          description: "{{$labels.instance}}: CPU usage is above 80%"
          value: "{{$value}}"