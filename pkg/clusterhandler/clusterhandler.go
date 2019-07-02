/*
Copyright 2019 Baidu, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package clusterhandler watchs k8s crd if it is enabled,
// listen on a websocket tunnel to access connection from child,
// and process message from child or parent.
package clusterhandler

import (
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	clusterrouter "github.com/baidu/ote-stack/pkg/clusterrouter"
	"github.com/baidu/ote-stack/pkg/clusterselector"
	"github.com/baidu/ote-stack/pkg/config"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

var (
	mergeToApiserverMutex = &sync.Mutex{}
)

// ClusterHandler is the interface to do cluster handler job.
// Get one by NewClusterHandler and Start it.
type ClusterHandler interface {
	Start() error // nonblock
}

type clusterHandler struct {
	conf                 *config.ClusterControllerConfig
	tunn                 tunnel.CloudTunnel
	clusterCRD           *k8sclient.ClusterCRD
	clusterControllerCRD *k8sclient.ClusterControllerCRD
	k8sEnable            bool
}

// NewClusterHandler news a ClusterHandler by ClusterControllerConfig.
func NewClusterHandler(c *config.ClusterControllerConfig) (ClusterHandler, error) {
	ch := &clusterHandler{
		conf:      c,
		k8sEnable: false,
	}
	if err := ch.valid(); err != nil {
		return nil, err
	}
	tunn := tunnel.NewCloudTunnel(c.TunnelListenAddr)
	if tunn == nil {
		return nil, fmt.Errorf("tunnel is nil with no error, listen addr is " + c.TunnelListenAddr)
	}
	tunn.RegistCheckNameValidFunc(ch.checkClusterName)
	tunn.RegistReturnMessageFunc(ch.handleMessageFromChild)
	tunn.RegistClientCloseHandler(ch.closeChild)
	tunn.RegistAfterConnectHook(ch.afterClusterConnect)
	ch.tunn = tunn
	return ch, nil
}

// valid check if config of cluster handler is valid, return error if it is invalid.
// call before Start.
func (c *clusterHandler) valid() error {
	if c.conf.ClusterName == "" {
		return fmt.Errorf("cluster name of cluster controller cannot be empty, set by --cluster-name")
	}
	if c.conf.ParentCluster == "" && !c.isRoot() {
		return fmt.Errorf("root cluster(no parent-cluster set) should not set cluster name")
	}
	if c.conf.ParentCluster != "" && c.isRoot() {
		return fmt.Errorf("no-root cluster must set cluster name(cannot be same as root cluster)")
	}
	if c.conf.TunnelListenAddr == "" {
		return fmt.Errorf("listen tunn is empty, listen addr is " + c.conf.TunnelListenAddr)
	}
	// if it is root, must connect to k8s
	if c.isRoot() {
		if c.conf.K8sClient == nil {
			return fmt.Errorf("k8s client cannot be nil in root, check kubeconfig")
		}
		c.k8sEnable = true
	} else {
		if c.conf.K8sClient != nil {
			c.k8sEnable = true
		}
	}
	if c.k8sEnable {
		klog.Infof("k8s enable")
		// init crd
		c.clusterCRD = k8sclient.NewClusterCRD(c.conf.K8sClient)
		if c.clusterCRD == nil {
			return fmt.Errorf("cluster crd not init in root, please check kubeconfig")
		}
		c.clusterControllerCRD = k8sclient.NewClusterControllerCRD(c.conf.K8sClient)
		if c.clusterControllerCRD == nil {
			return fmt.Errorf("cluster controller crd not init in root, please check kubeconfig")
		}
	}
	return nil
}

// Start run cluster handler.
// 1. listen cloud tunnel,
// 2. handle message from parent,
// 3. if k8s is configed, watch clustercontroller crd.
func (c *clusterHandler) Start() error {
	// start listen tunnel
	if err := c.tunn.Start(); err != nil {
		return err
	}

	// handle message from parent
	go c.handleMessageFromParent()

	// watch k8s apiserver for clustercontroller crd if k8s is enable
	if c.k8sEnable {
		factory := oteinformer.NewSharedInformerFactoryWithOptions(c.conf.K8sClient,
			config.K8S_INFORMAER_SYNC_DURATION*time.Second,
			oteinformer.WithNamespace(otev1.CLUSTER_NAMESPACE))
		informer := factory.Ote().V1().ClusterControllers().Informer()
		// actually, gracefull stop is not supported
		stopper := make(chan struct{})
		// add handler
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ca := obj.(*otev1.ClusterController)
				klog.V(3).Infof("clustercontroller add %v", ca)
				c.addClusterController(ca)
			},
		})
		go informer.Run(stopper)
	}

	// TODO if this is root, regist self to etcd
	// if there is already a root, start failed
	// handle kill signal, delete root cluster info if root has been killed

	return nil
}

/*
addClusterController is k8s cluster controller crd watch AddFunc.
1. tag parent name as self cluster name,
2. send to child.
*/
func (c *clusterHandler) addClusterController(cc *otev1.ClusterController) {
	// check if crd is valid to process, drop it if invalid
	if !hasToProcessClusterController(cc) {
		return
	}
	// add parentClusterName
	cc.Spec.ParentClusterName = c.conf.ClusterName
	// send to child
	// directed broadcast by cluster selector
	selectedChild := selectChild(cc)
	for port, portCC := range selectedChild {
		klog.V(3).Infof("send %v to %s with selector %s", portCC, port, portCC.Spec.ClusterSelector)
		c.sendToChild(portCC, port)
	}

	// broadcast to all childs if do not use selector
	// c.sendToChild(cc)
}

func selectChild(cc *otev1.ClusterController) map[string]*otev1.ClusterController {
	selector := clusterselector.NewSelector(cc.Spec.ClusterSelector)
	subtreeClusters := clusterrouter.Router().SubTreeClusters()
	var selectedSubTreeClusters []string
	ret := make(map[string]*otev1.ClusterController)
	for _, subtreeCluster := range *subtreeClusters {
		if selector.Has(subtreeCluster) {
			selectedSubTreeClusters = append(selectedSubTreeClusters, subtreeCluster)
		}
	}
	// get out ports of selected subtree clusters
	portsToSubtreeClusters := clusterrouter.Router().PortsToSubtreeClusters(&selectedSubTreeClusters)
	for port, subtree := range *portsToSubtreeClusters {
		portCC := cc.DeepCopy()
		portCC.Spec.ClusterSelector = clusterselector.ClustersToSelector(&subtree)
		ret[port] = portCC
	}
	return ret
}

/*
hasToProcessClusterController check if cluster controller crd is valid to process.
process clustercontroller crd added in 1 hour and has no response.
*/
func hasToProcessClusterController(ca *otev1.ClusterController) bool {
	createTime := ca.ObjectMeta.CreationTimestamp
	if !createTime.Add(1 * time.Hour).After(time.Now()) {
		klog.V(1).Infof("clustercontroller %s created 1 hour ago", ca.ObjectMeta.Name)
		return false
	}
	if len(ca.Status) == 0 {
		klog.V(1).Infof("clustercontroller %s has no response, do it", ca.ObjectMeta.Name)
		return true
	}
	klog.V(1).Infof("clustercontroller %s(%s) has response, do not do it", ca.ObjectMeta.Name, ca.ObjectMeta.Namespace)
	return false
}

/*
sendToChild send ClusterController to child.
if tos is not empty, send cc to them,
otherwise, broadcast message to all child.
*/
func (c *clusterHandler) sendToChild(cc *otev1.ClusterController, tos ...string) {
	if cc == nil {
		klog.Errorf("message send to child is nil")
		return
	}
	data, err := cc.Serialize()
	if err != nil {
		klog.Errorf("serialize clustercontroller crd(%v) failed: %v", cc, err)
		return
	}
	if len(tos) == 0 {
		go c.tunn.Broadcast(data)
	} else {
		for _, to := range tos {
			go c.tunn.Send(to, data)
		}
	}
}

/*
handleMessageFromParent handler message from parent(edge handler of this process).
*/
func (c *clusterHandler) handleMessageFromParent() {
	for {
		cc := <-c.conf.EdgeToClusterChan
		// if it is a route message from parent, update route
		// otherwise, send to child
		if cc.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_CLUSTER_ROUTE {
			clusterrouter.UpdateRouter(&cc, c.sendToChild)
		} else {
			// directed broadcast by cluster selector
			selectedChild := selectChild(&cc)
			for port, portCC := range selectedChild {
				klog.V(3).Infof("send %v to %s with selector %s", portCC, port, portCC.Spec.ClusterSelector)
				c.sendToChild(portCC, port)
			}

			// broadcast to all childs if do not use selector
			// c.sendToChild(cc)
		}
	}
}

/*
checkClusterName runs before stablish a connection to a child.
regist to cloud tunnel before Start it.
*/
func (c *clusterHandler) checkClusterName(cr *config.ClusterRegistry) bool {
	cl := otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: otev1.CLUSTER_NAMESPACE,
		},
		Spec: otev1.ClusterSpec{
			Name:   cr.Name,
			Listen: cr.Listen,
		},
		Status: otev1.ClusterStatus{
			Status:    otev1.CLUSTER_STATUS_REGIST,
			Timestamp: time.Now().Unix(),
		},
	}
	cc, err := cl.WrapperToClusterController(otev1.CLUSTER_CONTROLLER_DEST_REGIST_CLUSTER)
	if err != nil {
		klog.Errorf("%v", err)
		return false
	}

	// handle a cluster regist message
	go c.handleRegistClusterMessage(cr.Name, cc)
	return true
}

/*
afterClusterConnect runs after connect to a child.
*/
func (c *clusterHandler) afterClusterConnect(cr *config.ClusterRegistry) {
	// add cluster to route
	clusterrouter.Router().AddChild(cr.Name, cr.Listen, c.sendToChild)
}

/*
handleMessageFromChild handle msg from child.
There are things to do with message from child.
1. it is a cluster-regist message, handle registClusteregist message,
2. the parentClusterName of that message equals to self name, merge to apiserver,
3. otherwise, transmit to parent.
*/
func (c *clusterHandler) handleMessageFromChild(client string, msg []byte) (ret error) {
	ret = nil
	cc, err := otev1.ClusterControllerDeserialize(msg)
	if err != nil {
		ret = fmt.Errorf("deserialize clustercontroller(%s) failed: %v", string(msg), err)
		klog.Error(ret)
		return
	}
	// if the msg has no parentClusterName, set it to self
	if cc.Spec.ParentClusterName == "" {
		cc.Spec.ParentClusterName = c.conf.ClusterName
	}

	if cc.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_REGIST_CLUSTER {
		ret = c.handleRegistClusterMessage(client, cc)
	} else if cc.Spec.Destination == otev1.CLUSTER_CONTROLLER_DEST_UNREGIST_CLUSTER {
		ret = c.handleUnregistClusterMessage(client, cc)
	} else if cc.Spec.ParentClusterName == c.conf.ClusterName {
		ret = c.mergeToApiserver(cc)
	} else {
		c.transmitToParent(cc)
	}

	return
}

/*
isRoot checks whether a cluster is root.
*/
func (c *clusterHandler) isRoot() bool {
	return config.IsRoot(c.conf.ClusterName)
}

/*
handleRegistClusterMessage handle a cluster-regist message.
once get a regist message, a cluster should do things below:
1. TODO check if the cluster name is valid,
2. write cluster info to k8s apiserver if self is root, else transmit to parent,
3. record cluster router.
*/
func (c *clusterHandler) handleRegistClusterMessage(
	client string, cc *otev1.ClusterController) (ret error) {
	ret = nil
	cluster := getClusterFromClusterController(cc)
	if cluster == nil {
		ret = fmt.Errorf("regist message cannot get cluster info")
		klog.Error(ret)
		return
	}

	clusterrouter.Router().AddRoute(cluster.Spec.Name, client)

	if c.isRoot() {
		// TODO handle rename situation
		c.clusterCRD.Create(cluster)
	} else {
		c.transmitToParent(cc)
	}

	return
}

/*
closeChild runs when a child disconnect to this cluster.
1. delete child cluster from whole,
2. if this is a root, remove cluster from etcd,
3. delete child from route and update route to other childs.
*/
func (c *clusterHandler) closeChild(clusterName string) {
	// delete child from route
	clusterrouter.Router().DelChild(clusterName, c.sendToChild)

	// if this is root, delete cluster from etcd
	// otherwise, report unregist to root
	cl := otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: otev1.CLUSTER_NAMESPACE,
		},
		Spec: otev1.ClusterSpec{
			Name: clusterName,
		},
	}
	cbyte, err := cl.Serialize()
	if err != nil {
		klog.Errorf("serialize cluster crd(%v) failed: %v", cl, err)
		return
	}
	cc := otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			Destination: otev1.CLUSTER_CONTROLLER_DEST_UNREGIST_CLUSTER,
			Body:        string(cbyte),
		},
	}
	go c.handleUnregistClusterMessage(clusterName, &cc)
}

/*
handleUnregistClusterMessage handle a cluster unregist message.
1. if this is root, delete cluster crd from etcd,
otherwise, transmit to parent,
2. modify route.
*/
func (c *clusterHandler) handleUnregistClusterMessage(
	client string, cc *otev1.ClusterController) (ret error) {
	ret = nil
	cluster := getClusterFromClusterController(cc)
	if cluster == nil {
		ret = fmt.Errorf("unregist message cannot get cluster info")
		klog.Error(ret)
		return
	}

	clusterrouter.Router().DelRoute(cluster.Spec.Name, client)

	if c.isRoot() {
		c.clusterCRD.Delete(cluster)
	} else {
		c.transmitToParent(cc)
	}

	return
}

/*
mergeToApiserver merge response to etcd with mutex lock.
cc is part of response to a cluster controller crd reqeust.
*/
func (c *clusterHandler) mergeToApiserver(cc *otev1.ClusterController) error {
	mergeToApiserverMutex.Lock()
	defer mergeToApiserverMutex.Unlock()
	// get clustercontroller crd by name
	if origin := c.clusterControllerCRD.Get(cc.ObjectMeta.Namespace, cc.ObjectMeta.Name); origin != nil {
		// merge status and update timestamp
		new := origin.DeepCopy()
		if new.Status == nil {
			new.Status = make(map[string]otev1.ClusterControllerStatus)
		}
		for cn, s := range cc.Status {
			if originStatus, ok := origin.Status[cn]; !ok {
				new.Status[cn] = s
			} else {
				// update cluster status if timestamp is new
				if originStatus.Timestamp < s.Timestamp {
					new.Status[cn] = s
				}
			}
		}
		// update new to apiserver
		klog.Infof("crd reponse update %s-%s", new.ObjectMeta.Namespace, new.ObjectMeta.Name)
		c.clusterControllerCRD.Update(new)
	}
	return nil
}

/*
transmitToParent transmit message to parent asynchronously.
*/
func (c *clusterHandler) transmitToParent(cc *otev1.ClusterController) {
	go func() {
		c.conf.ClusterToEdgeChan <- *cc
	}()
}

func getClusterFromClusterController(cc *otev1.ClusterController) *otev1.Cluster {
	// deserialize cluster
	cluster, err := otev1.ClusterDeserialize([]byte(cc.Spec.Body))
	if err != nil {
		klog.Errorf("deserialize cluster(%s) failed: %v", cc.Spec.Body, err)
		return nil
	}
	return cluster
}
