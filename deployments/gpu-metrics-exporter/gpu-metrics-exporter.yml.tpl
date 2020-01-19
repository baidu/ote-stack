apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: gpu-metrics-exporter
  namespace: monitor
spec:
  template:
    metadata:
      labels:
        app: gpu-metrics-exporter
      name: gpu-metrics-exporter
    spec:
      hostNetwork: true
      nodeSelector:
        gpu: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      containers:
      - image: _HARBOR_IMAGE_ADDR_/pod-gpu-metrics-exporter:v1.0.0-alpha
        name: pod-nvidia-gpu-metrics-exporter
        ports:
        - name: gpu-metrics
          containerPort: 9400
        securityContext:
          runAsNonRoot: false
          runAsUser: 0
        volumeMounts:
        - name: pod-gpu-resources
          readOnly: true
          mountPath: /var/lib/kubelet/pod-resources
        - name: device-metrics
          readOnly: true
          mountPath: /run/prometheus
      - image: _HARBOR_IMAGE_ADDR_/dcgm-exporter:1.4.6
        name: nvidia-dcgm-exporter
        securityContext:
          runAsNonRoot: false
          runAsUser: 0
        volumeMounts:
        - name: device-metrics
          mountPath: /run/prometheus
      volumes:
      - name: pod-gpu-resources
        hostPath:
          path: /var/lib/kubelet/pod-resources
      - name: device-metrics
        emptyDir:
          medium: Memory
---

apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: nvidia-device-plugin
  namespace: monitor
spec:
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        app: nvidia-device-plugin
    spec:
      nodeSelector:
        gpu: deploy
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      tolerations:
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      containers:
      - image: _HARBOR_IMAGE_ADDR_/k8s-device-plugin:1.11
        name: nvidia-device-plugin-ctr
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        volumeMounts:
          - name: device-plugin
            mountPath: /var/lib/kubelet/device-plugins
      volumes:
        - name: device-plugin
          hostPath:
            path: /var/lib/kubelet/device-plugins
