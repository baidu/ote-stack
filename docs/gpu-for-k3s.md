# Nvidia GPU supporting in K3S

## Motivation
This article aims at guiding how to support Nvidia GPU in K3S, including embedded containerd using the gnu runtime and scheduling gpu resources in K3S.

## Prerequisites
This guide assumes you’ve got hardware (or a VM) with a CUDA enabled Nvidia graphics card and that you’re running Unix-like OS. The verification is using K3S' 1.17.3 version. It may has some differences in other versions.

## Embedded containerd support GPU
### Generating default configure of containerd
First, find the containerd binary executable file. It can be found in data directory of k3s agent which is defined by “--data-dir” args passed in. If not defined, it will be set to “/var/lib/rancher/k3s” as default. In ${data_dir}/data, There is an only directory named with a long string and the binary file is located in ./bin in it.
``` shell
[root@mytest data]# pwd
/var/lib/rancher/k3s/data
[root@mytest data]#  cd data/7c4aaa633ac3ff4849166ba2759da158a70beb5734940e84b6e67011a35f4c59/bin  
[root@mytest bin]# ll containerd
-rwxr-xr-x 1 root root 110537304 1月  28 02:09 containerd
```

Secondly, generate the default configure by the executable contained file to ‘${data_dir}/agent/etc/containerd’ .
```shell
[root@mytest bin]# cd 7c4aaa633ac3ff4849166ba2759da158a70beb5734940e84b6e67011a35f4c59
[root@mytest 7c4aaa633ac3ff4849166ba2759da158a70beb5734940e84b6e67011a35f4c59]# ./containerd config default > /var/lib/rancher/k3s/agent/data/etc/containerd/config.toml.tmpl
[root@mytest bin]# ll /var/lib/rancher/k3s/agent/data/etc/containerd/config.toml.tmpl
-rw-r--r-- 1 root root 3403 3月  13 15:10 /var/lib/rancher/k3s/data/agent/etc/containerd/config.toml.tmpl
```

### Modify the configure
After generating the default configure, the next is to make embedded containerd aware of the GPU runtime. This can be achieved by modifying the configure.

By reading the source code of containerd, “io.containerd.runtime.v1.linux” is the only runtime type which support the self-defined runtime plugin. So the default contained configure must be make changes as below
``` shell
    …
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runtime.v1.linux"
…
  [plugins."io.containerd.runtime.v1.linux"]
    runtime = "nvidia-container-runtime"
…
```
The runtime is set to "nvidia-container-runtime" so that contained can use nvidia GPU runtime as the default runtime.

### Other modifications
All above we do is around the configure template of containerd but in fact the default config is not the one. It must be adjusted to a template file that K3S can recognize. You can get the template from [config.toml.tmpl](../deployments/k3s/config.toml.tmpl) 

### Restart K3S agent
After restarting, check if there is a new config generated from the self-define config template.

## Deploy Nvidia GPU device plugin
Notice: K3S support the GPU only in 1.17.3 or later
Get it from  [config.toml.tmpl](../deployments/k3s/gpu_device_plugin.yml)  and apply it.
```shell
k3s kubectl apply -f gpu_device_plugin.yml
```

Check the GPU node has allocatable GPU resource.
```shell
[root@mytest script]# k3s kubectl describe node gpu-node1
Name:               gpu-node1
Roles:              <none>
Labels:             beta.kubernetes.io/arch=amd64
                    beta.kubernetes.io/os=linux
                    kubernetes.io/arch=amd64
                    kubernetes.io/hostname=gpu-node1
                    kubernetes.io/os=linux
Annotations:        node.alpha.kubernetes.io/ttl: 15
                    volumes.kubernetes.io/controller-managed-attach-detach: true
CreationTimestamp:  Fri, 13 Mar 2020 16:39:57 +0800
Taints:             <none>
Unschedulable:      false
Lease:
  HolderIdentity:  gpu-node1
  AcquireTime:     <unset>
  RenewTime:       Fri, 13 Mar 2020 16:40:17 +0800
Conditions:
  Type             Status  LastHeartbeatTime                 LastTransitionTime                Reason                       Message
  ----             ------  -----------------                 ------------------                ------                       -------
  MemoryPressure   False   Fri, 13 Mar 2020 16:40:07 +0800   Fri, 13 Mar 2020 16:39:56 +0800   KubeletHasSufficientMemory   kubelet has sufficient memory available
  DiskPressure     False   Fri, 13 Mar 2020 16:40:07 +0800   Fri, 13 Mar 2020 16:39:56 +0800   KubeletHasNoDiskPressure     kubelet has no disk pressure
  PIDPressure      False   Fri, 13 Mar 2020 16:40:07 +0800   Fri, 13 Mar 2020 16:39:56 +0800   KubeletHasSufficientPID      kubelet has sufficient PID available
  Ready            True    Fri, 13 Mar 2020 16:40:07 +0800   Fri, 13 Mar 2020 16:40:07 +0800   KubeletReady                 kubelet is posting ready status
Addresses:
  InternalIP:  10.159.11.168
  Hostname:    gpu-node1
Capacity:
  cpu:                80
  ephemeral-storage:  14189488Ki
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  memory:             263784628Ki
  nvidia.com/gpu:     4
  pods:               110
```

## Testing
Deploy a pod and attach it. Execute the command of “nvidia-smi”
```shell
root@0371612019052# nvidia-smi
Mon Mar 16 11:24:27 2020      
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 418.39       Driver Version: 418.39       CUDA Version: 10.1     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|===============================+======================+======================|
|   0  Tesla P4            On   | 00000000:3B:00.0 Off |                    0 |
| N/A   35C    P8     7W /  75W |      0MiB /  7611MiB |      0%      Default |
+-------------------------------+----------------------+----------------------+
|   1  Tesla P4            On   | 00000000:86:00.0 Off |                    0 |
| N/A   37C    P8     6W /  75W |      0MiB /  7611MiB |      0%      Default |
+-------------------------------+----------------------+----------------------+
|   2  Tesla P4            On   | 00000000:AF:00.0 Off |                    0 |
| N/A   46C    P0    23W /  75W |   2196MiB /  7611MiB |      0%      Default |
+-------------------------------+----------------------+----------------------+
|   3  Tesla P4            On   | 00000000:D8:00.0 Off |                    0 |
| N/A   34C    P8     6W /  75W |      0MiB /  7611MiB |      0%      Default |
+-------------------------------+----------------------+----------------------+
                                                                                
+-----------------------------------------------------------------------------+
| Processes:                                                       GPU Memory |
|  GPU       PID   Type   Process name                             Usage      |
|=============================================================================|
|    2     57256      C   bin/feature_service                         2186MiB |
+-----------------------------------------------------------------------------+

```
It is OK If it show like this.
