# OTE-Stack

[![Build Status](https://travis-ci.org/baidu/ote-stack.svg?branch=master)](https://travis-ci.org/baidu/ote-stack)
[![Go Report Card](https://goreportcard.com/badge/github.com/baidu/ote-stack)](https://goreportcard.com/report/github.com/baidu/ote-stack)
[![GoDoc](https://godoc.org/github.com/baidu/ote-stack?status.svg)](https://godoc.org/github.com/baidu/ote-stack)

OTE-Stack is an edge computing platform for 5G and AI. By virtualization it can shield heterogeneous characteristics and gives a unified access of cloud edge, mobile edge and private edge. For AI it provides low-latency, high-reliability and cost-optimal computing support at the edge through the cluster management and intelligent scheduling of multi-tier clusters. And at the same time OTE-Stack makes device-edge-cloud collaborative computing possible.

**Note:** OTE-Stack is a heavy work in progress.

## Advantages

### Large scale and hierarchical cluster management
Through the standard interface, hierarchical clusters can be built quickly. The number of clusters can be theoretically unlimited which can effectively solve the management and scheduling problems of large-scale mobile edge clusters in 5G era.

### Support third cluster
It supports [kubernetes](https://github.com/kubernetes/) and [k3s](https://github.com/rancher/k3s) now. Because edge is a logical cluster, it can support any clusters by cluster-shim in theory. So in 5G era, it can be compatible with different implementations of different operators'MEC platforms.

### Lightweight cluster controller 
Only one component and one customize shim can make the third cluster controlled by OTE-Stack. So it's very light and easy to use.

### Cluster autonomy
The edge is a complete logical cluster which caches almost all the states. So it will run normally when it is disconnected from the center which can effectively solve the problem of cluster autonomy in the case of weak edge networks.

### Automatic disaster recovery
Because of the hierarchical design of clusters, Cluster Controller at each level will automatically acquire the alternate nodes. Once the connection with the parent node is lost, the connection with the center will be restored through the alternate nodes.

### Global scheduling
Through Cluster Controller, all the clusters can be integrated into the unified scheduling, and the global optimal use of edge resources can be achieved.

### Support multi-runtimes
OTE-Stack leverages [virtlet](https://github.com/Mirantis/virtlet) for VM-based workloads, and also adds VM operation(start, stop, mount, etc.) via CustomResourceDefinition. So it supports VM, Kata containers and runc which can orchestrate in a unified way.

### Kubernetes native support
With OTE-Stack, users can orchestrate dockers/VM on Edge clusters just like a traditional kubernetes cluster in the Cloud.

## Architecture
[![architecture](./docs/images/architecture.png)](./docs/images/architecture.png)

OTE-Stack features a pluggable architecture, making it much easier to build on.
* The global scheduler is fully compatible with kubernetes. Users can operate directly using kubectl;
* Using websocket for the edge-cloud communication;
* In addition to the cluster name, the cluster tag can be added customically. Cluster tag matching through intelligent cluster-selecter to achieve accurate routing of messages;
* Through k8s-cluster-shim to achieve the management of kubernetes cluster, shielding the native implementation within the kubernetes cluster;
* According to the interface of OTE-Stack, the cluster shim of the third party cluster can be realized to access and schedule the third party cluster. The internal implementation of the third party cluster is shielded;
* Each layer can be used as a control entry to control all sub-clusters below this layer. Users can also use kubectl or API to implement custom cluster management and scheduling.

### Components
 * WebFrontend
 * WebBackend
 * OpenAPI
 * Scheduler
 * ChartManager
 * **EdgeTunnel**
 > Northbound interface of Controller. By establishing websocket connection with **CloudTunnel** of upper cluster, messages between clusters can be transmitted smoothly.
 * **EdgeHandler**
 > It can add tags to cluster, receive and process messages from upper cluster, transmit messages to **ClusterHandler**, receive messages from **ClusterHandler** and realize cluster disaster recovery automatically.
   * Users can configure their own cluster name or add cluster tags to achieve complex cluster management.
   * Used for receiving messages sent by **EdgeTunnel** and forwarding them to **Cluster Selecter** for routing or direct transmission to **ClusterHandler** after processing.
   * Receive messages sent back by **ClusterHandler** or shim (such as changes in sub-cluster, status, etc.) and pass them to the upper cluster through **EdgeTunnel** after processing.
   * Once the connection between the current cluster and the parent cluster is established, the sibling cluster of the parent, the parent cluster of the parent and the sibling cluster of itself will be automatically acquired as the alternative cluster. When Disconnected, the alternative one is connected automatically. The connection to the central can be quickly restored. Meanwhile, it regularly checks whether the previous parent cluster is restored, and once restored, it restores the previous connection topology.
 * **ClusterSelecter**
 > It is used to complete the routing of cluster messages, and it accepts the processing of two kinds of cluster routing rules.
   * If it is a real list of cluster names, it matches the names according to the cluster routing rules and looks for the next hop until it reaches the specified cluster accurately.
   * If it's a cluster's fuzzy rules, such as\* tagA*, it matches all tagA-containing clusters in the tag and maps them to the real names of the clusters. Then it uses the above rules to pass down until it reaches the specified cluster accurately.
 * **ClusterHandler**
 > It's core components of cluster management.
   * Store the names and labels of all subclusters.
   * Establish routing rules that store the next hop cluster name to any sub-cluster to support accurate delivery of messages.
   * Notify the upper cluster in time when the sub-cluster changes (such as disconnection, status updates, etc.)
 * **CloudTunnel**
 > Southbound interface of Controller. By establishing websocket connection with **EdgeTunnel** of sub-cluster, messages between clusters can be transmitted smoothly.
 * **k8s-cluster-shim**
 > It is an adapter of kubernetes cluster, which receives and parses cluster messages forwarded by **OTE Cluster Controller**, sends them to kubernetes cluster for corresponding processing, and returns the results and status to **OTE Cluster Controller** in time.
 * **k3s-cluster-shim**
 > It is an adapter of k3s cluster, which receives and parses cluster messages forwarded by **OTE Cluster Controller**, sends them to k3s cluster for corresponding processing, and returns the results and status to **OTE Cluster Controller** in time.
 * **NodeAgent**
 > It is deployed on edge nodes to retrieve data from **cAdvisor** and **Node-Exporter** which will be uploaded to **NodesServer** in edge clusters.
 * **NodesServer**
 > In the edge cluster, it is used to aggregate data of each node and provide it to **Prometheus** (Prometheus can also directly collect data of the node, but requires the node to open the corresponding ports)
 * **DataQueryServer**
 > Exposing Prometheus data as APIs to OpenAPI and Scheduler
 * **VMController**
 > Operations for a single VM, such as start, stop, etc. 

## Current Features
* Hierarchical cluster management
* Support kubernetes and k3s cluster by given shim
* Duplex channel from center to edge cluster
* Cluster autonomy
* Automatic disaster recovery(Previous topology is not yet restored)
* Kubernetes native support and it's optional choice
* Accurate routing of messages between clusters

## Getting Started
[Quick Start](./docs/quickstart.md)

## TODO
OTE-Stack aims to providing a complete solution to edge cloud, so we will gradually open and improve the following core functions in the future.

#### Add Cluster Tags
By adding some tags for edge cluster, you can achieve complex cluster management.

#### Lightweight Edge Cluster
Although k3s and [kubeedge](https://github.com/kubeedge/kubeedge) have provided reference designs for edge clusters and their design ideas are ingenious, we will still explore how lightweight clusters suitable for edge infrastructure can be designed to better support edge clouds.

#### Edge Cluster Interface Standard
We need to consider the scheme of edge cluster and the design of 5G MEC(Multi-access Edge Computing) to abstract the interface and form the access standard of edge cluster. Now provider interface of [virtual kubelet](https://github.com/virtual-kubelet/virtual-kubelet) may be a candidate for us and we'll try to see if it meets all our requirements.

#### Global Scheduling Strategy
Because the realization of sub-clusters is different, we need to consider how to use limited data on the basis of hierarchical clusters to schedule resources reasonably to meet the purpose of maximizing the utilization of edge cloud resources.

#### Message Routing Policy
In order to ensure the accurate delivery of messages, it is necessary to design the shortest path algorithm combining with the topological rules to reduce the transmission of useless messages.

We will also focus on and provide reference designs for other modules, such as Micro-Service framework, automation operation and maintenance of large-scale edge cluster, multi-tenant for edge cloud and Device-Edge-Cloud cooperation.

## Contributing
If you're interested in being a contributor and want to get involved in developing the OTE-Stack code, please see [CONTRIBUTING](./CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

## Discussion
Email: ote-stack@baidu.com

## License
OTE-Stack is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
