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

	"github.com/golang/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/clusterrouter"
	"github.com/baidu/ote-stack/pkg/clusterselector"
	"github.com/baidu/ote-stack/pkg/config"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions"
	"github.com/baidu/ote-stack/pkg/k8sclient"
	"github.com/baidu/ote-stack/pkg/tunnel"
)

const (
	controllerManagerChanBufferSize = 100
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
	// msg from clusters back to controller manager
	backToControllerManagerChan chan clustermessage.ClusterMessage
	// msg from controller manager to publish to clusters
	controllerManagerPublishChan chan clustermessage.ClusterMessage
}

// NewClusterHandler news a ClusterHandler by ClusterControllerConfig.
func NewClusterHandler(c *config.ClusterControllerConfig) (ClusterHandler, error) {
	ch := &clusterHandler{
		conf:      c,
		k8sEnable: false,
		backToControllerManagerChan: make(chan clustermessage.ClusterMessage,
			controllerManagerChanBufferSize),
		controllerManagerPublishChan: make(chan clustermessage.ClusterMessage,
			controllerManagerChanBufferSize),
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
	tunn.RegistControllerManagerMsgHandler(ch.controllerMsgHandler)
	ch.tunn = tunn
	return ch, nil
}

// valid check if config of cluster handler is valid, return error if it is invalid.
// call before Start.
func (c *clusterHandler) valid() error {
	if c.conf.ClusterUserDefineName == "" {
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
			config.K8sInformerSyncDuration*time.Second,
			oteinformer.WithNamespace(otev1.ClusterNamespace))
		informer := factory.Ote().V1().ClusterControllers().Informer()
		// actually, graceful stop is not supported
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
	// transfer crd to cluster message
	msg := clusterControllerCRDToClusterMessage(cc, clustermessage.CommandType_ControlReq)
	if msg == nil {
		klog.Errorf("cluster msg is nil when add a crd %v", cc)
		return
	}
	// send to child
	// directed broadcast by cluster selector
	selectedChild := selectChild(msg)
	for port, portMsg := range selectedChild {
		klog.V(3).Infof("send %v to %s with selector %s", portMsg, port, portMsg.Head.ClusterSelector)
		c.sendToChild(portMsg, port)
	}

	// broadcast to all childs if do not use selector
	// c.sendToChild(msg)
}

func selectChild(msg *clustermessage.ClusterMessage) map[string]*clustermessage.ClusterMessage {
	selector := clusterselector.NewSelector(msg.Head.ClusterSelector)
	subtreeClusters := clusterrouter.Router().SubTreeClusters()
	var selectedSubTreeClusters []string
	ret := make(map[string]*clustermessage.ClusterMessage)
	for _, subtreeCluster := range subtreeClusters {
		if selector.Has(subtreeCluster) {
			selectedSubTreeClusters = append(selectedSubTreeClusters, subtreeCluster)
		}
	}
	// get out ports of selected subtree clusters
	portsToSubtreeClusters := clusterrouter.Router().PortsToSubtreeClusters(&selectedSubTreeClusters)
	for port, subtree := range portsToSubtreeClusters {
		portMsg := proto.Clone(msg).(*clustermessage.ClusterMessage)
		portMsg.Head.ClusterSelector = clusterselector.ClustersToSelector(&subtree)
		ret[port] = portMsg
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
func (c *clusterHandler) sendToChild(msg *clustermessage.ClusterMessage, tos ...string) {
	if msg == nil {
		klog.Errorf("message send to child is nil")
		return
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		klog.Errorf("serialize cluster message(%v) failed: %v", msg, err)
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
		msg := <-c.conf.EdgeToClusterChan
		// if it is a route message from parent, update route
		// otherwise, send to child
		if msg.Head.Command == clustermessage.CommandType_NeighborRoute {
			clusterrouter.UpdateRouter(&msg, c.sendToChild)
		} else {
			// directed broadcast by cluster selector
			selectedChild := selectChild(&msg)
			for port, portMsg := range selectedChild {
				klog.V(3).Infof("send %v to %s with selector %s", portMsg, port, portMsg.Head.ClusterSelector)
				c.sendToChild(portMsg, port)
			}

			// broadcast to all childs if do not use selector
			// c.sendToChild(msg)
		}
	}
}

/*
checkClusterName runs before stablish a connection to a child.
regist to cloud tunnel before Start it.
*/
func (c *clusterHandler) checkClusterName(cr *config.ClusterRegistry) bool {
	if cr == nil {
		return false
	}

	cr.ParentName = c.conf.ClusterName
	cc, err := cr.WrapperToClusterMessage(clustermessage.CommandType_ClusterRegist)
	if err != nil {
		klog.Errorf("wrapper message for regist child failed: %v", err)
		return false
	}

	data, err := cc.Serialize()
	if err != nil {
		klog.Error(err)
		return false
	}

	// send to handler
	go c.handleMessageFromChild(cr.Name, data)

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
func (c *clusterHandler) handleMessageFromChild(client string, data []byte) (ret error) {
	ret = nil
	msg := &clustermessage.ClusterMessage{}
	err := proto.Unmarshal(data, msg)
	if err != nil {
		ret = fmt.Errorf("deserialize cluster message(%s) failed: %v", string(data), err)
		klog.Error(ret)
		return
	}
	if msg.Head == nil {
		ret = fmt.Errorf("deserialize cluster message(%s) failed: message head is nil", string(data))
		klog.Error(ret)
		return
	}
	// if the msg has no parentClusterName, set it to self
	if msg.Head.ParentClusterName == "" {
		msg.Head.ParentClusterName = c.conf.ClusterName
	}

	switch msg.Head.Command {
	case clustermessage.CommandType_ClusterRegist:
		if c.isRoot() {
			ret = c.sendToControllerManager(msg)
		}
		// TODO do not access k8s in cluster controller
		ret = c.handleRegistClusterMessage(client, msg)
	case clustermessage.CommandType_ClusterUnregist:
		if c.isRoot() {
			ret = c.sendToControllerManager(msg)
		}
		// TODO do not access k8s in cluster controller
		ret = c.handleUnregistClusterMessage(client, msg)
	case clustermessage.CommandType_SubTreeRoute:
		c.updateRouteToSubtree(msg)
	default:
		if c.isRoot() {
			// send to controller manager
			ret = c.sendToControllerManager(msg)
			// TODO return error if failed
			// TODO do not merge to apiserver
			ret = c.mergeToApiserver(msg)
		} else {
			c.transmitToParent(msg)
		}
	}
	return
}

func (c *clusterHandler) sendToControllerManager(msg *clustermessage.ClusterMessage) error {
	var ret error
	data, err := proto.Marshal(msg)
	if err != nil {
		ret = fmt.Errorf("serialize cluster message(%v) failed: %v", msg, err)
		klog.Error(ret)
		return ret
	}
	err = c.tunn.SendToControllerManager(data)
	if err != nil {
		ret = fmt.Errorf("send to controller manager failed: %v", err)
		klog.Error(ret)
		return ret
	}
	return nil
}

/*
isRoot checks whether a cluster is root.
*/
func (c *clusterHandler) isRoot() bool {
	return config.IsRoot(c.conf.ClusterUserDefineName)
}

/*
handleRegistClusterMessage handle a cluster-regist message.
once get a regist message, a cluster should do things below:
1. TODO check if the cluster name is valid,
2. write cluster info to k8s apiserver if self is root, else transmit to parent,
3. record cluster router.
*/
func (c *clusterHandler) handleRegistClusterMessage(
	client string, msg *clustermessage.ClusterMessage) (ret error) {
	ret = nil
	cr := getClusterRegistryFromClusterMessage(msg)
	cluster := getClusterFromClusterRegistry(cr)
	if cluster == nil {
		ret = fmt.Errorf("regist message cannot get cluster info")
		klog.Error(ret)
		return
	}

	// add the cluster to router
	// and if failed to add, do not transmit to parent or save to k8s
	err := clusterrouter.Router().AddRoute(cr.Name, client)
	if err != nil {
		// handle rename situation
		// TODO make the new one reconnect
		ret = fmt.Errorf("add route failed: %v", err)
		klog.Error(ret)
		return
	}

	if c.isRoot() {
		old := c.clusterCRD.Get(cluster.ObjectMeta.Namespace, cluster.ObjectMeta.Name)
		if old == nil {
			cluster.Status.Status = otev1.ClusterStatusOnline
			cluster.Status.Timestamp = time.Now().Unix()
			c.clusterCRD.Create(cluster)
		} else {
			// update cluster status to online
			old.Status.Status = otev1.ClusterStatusOnline
			old.Status.Timestamp = cluster.Status.Timestamp
			old.Status.Listen = cluster.Status.Listen
			old.Status.ParentName = cluster.Status.ParentName
			err = c.clusterCRD.UpdateStatus(old)
			if err != nil {
				ret = fmt.Errorf("update cluster status failed: %v", err)
				klog.Error(err)
				return
			}
		}
	} else {
		c.transmitToParent(msg)
	}

	return
}

/*
closeChild runs when a child disconnect to this cluster.
1. delete child cluster from whole,
2. if this is a root, remove cluster from etcd,
3. delete child from route and update route to other childs.
*/
func (c *clusterHandler) closeChild(cr *config.ClusterRegistry) {
	if cr == nil {
		return
	}

	cr.ParentName = c.conf.ClusterName
	// delete child from route
	clusterrouter.Router().DelChild(cr.Name, c.sendToChild)

	// if this is root, delete cluster from etcd
	// otherwise, report unregist to root
	cc, err := cr.WrapperToClusterMessage(clustermessage.CommandType_ClusterUnregist)
	if err != nil {
		klog.Errorf("wrapper message for close child failed: %v", err)
		return
	}
	data, err := cc.Serialize()
	if err != nil {
		klog.Error(err)
		return
	}

	// send to handler
	go c.handleMessageFromChild(cr.Name, data)
}

/*
handleUnregistClusterMessage handle a cluster unregist message.
1. if this is root, delete cluster crd from etcd,
otherwise, transmit to parent,
2. modify route.
*/
func (c *clusterHandler) handleUnregistClusterMessage(
	client string, msg *clustermessage.ClusterMessage) (ret error) {
	ret = nil
	cr := getClusterRegistryFromClusterMessage(msg)
	cluster := getClusterFromClusterRegistry(cr)
	if cluster == nil {
		ret = fmt.Errorf("unregist message cannot get cluster info")
		klog.Error(ret)
		return
	}

	clusterrouter.Router().DelRoute(cluster.ObjectMeta.Name, client)

	if c.isRoot() {
		old := c.clusterCRD.Get(cluster.ObjectMeta.Namespace, cluster.ObjectMeta.Name)
		if old != nil {
			// update to offline status
			old.Status.Status = otev1.ClusterStatusOffline
			old.Status.Timestamp = cr.Time
			err := c.clusterCRD.UpdateStatus(old)
			if err != nil {
				ret = fmt.Errorf("update cluster status failed: %v", err)
				klog.Error(err)
				return
			}
		}
	} else {
		c.transmitToParent(msg)
	}

	return
}

/*
mergeToApiserver merge response to etcd with mutex lock.
cc is part of response to a cluster controller crd reqeust.
*/
func (c *clusterHandler) mergeToApiserver(msg *clustermessage.ClusterMessage) error {
	mergeToApiserverMutex.Lock()
	defer mergeToApiserverMutex.Unlock()
	// transfer cluster message to crd
	cc := clusterMessageToClusterControllerCRD(msg)
	if cc == nil {
		return fmt.Errorf("transfer cluster message to crd failed")
	}
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
		klog.Infof("crd response update %s-%s", new.ObjectMeta.Namespace, new.ObjectMeta.Name)
		c.clusterControllerCRD.Update(new)
	}
	return nil
}

/*
transmitToParent transmit message to parent asynchronously.
*/
func (c *clusterHandler) transmitToParent(msg *clustermessage.ClusterMessage) {
	go func() {
		c.conf.ClusterToEdgeChan <- *msg
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

func getClusterRegistryFromClusterMessage(
	msg *clustermessage.ClusterMessage) *config.ClusterRegistry {
	cr, err := config.ClusterRegistryDeserialize(msg.Body)
	if err != nil {
		klog.Errorf("deserialize clusterregistry(%s) failed: %v", msg.Body, err)
		return nil
	}
	return cr
}

func getClusterFromClusterRegistry(cr *config.ClusterRegistry) *otev1.Cluster {
	if cr == nil {
		return nil
	}
	return &otev1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: otev1.ClusterNamespace,
		},
		Spec: otev1.ClusterSpec{
			Name: cr.UserDefineName,
		},
		Status: otev1.ClusterStatus{
			Listen:     cr.Listen,
			ParentName: cr.ParentName,
			Timestamp:  cr.Time,
		},
	}
}

// updateRouteToSubtree updates router to subtree of child.
func (c *clusterHandler) updateRouteToSubtree(msg *clustermessage.ClusterMessage) error {
	subtrees := clusterrouter.SubtreeFromClusterController(msg)
	if subtrees == nil {
		return fmt.Errorf("subtree route is empty")
	}
	var err error
	for to := range subtrees {
		err = clusterrouter.Router().AddRoute(to, msg.Head.ClusterName)
		if err != nil {
			klog.Errorf("add subtree router %s-%s failed: %v", to, msg.Head.ClusterName, err)
		}
	}
	// do not return part of err in for cycle
	return nil
}

func (c *clusterHandler) controllerMsgHandler(clientName string, data []byte) error {
	// unmarshal msg
	msg := &clustermessage.ClusterMessage{}
	err := proto.Unmarshal(data, msg)
	if err != nil {
		ret := fmt.Errorf("can not deserialize message from %s, error: %s", clientName, err.Error())
		klog.Error(ret)
		return ret
	}

	// check msg head
	if msg.Head == nil {
		err = fmt.Errorf("msg head is nil, cannot handle")
		klog.Errorf("%v", err)
		return err
	}
	// set parent cluster name
	if msg.Head.ParentClusterName == "" {
		msg.Head.ParentClusterName = c.conf.ClusterName
	}
	// send to downstream channel
	c.conf.EdgeToClusterChan <- *msg
	return nil
}

func clusterControllerCRDToClusterMessage(
	cc *otev1.ClusterController, command clustermessage.CommandType) *clustermessage.ClusterMessage {
	if cc == nil {
		return nil
	}
	ret := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			MessageID:         cc.ObjectMeta.Name,
			ClusterSelector:   cc.Spec.ClusterSelector,
			ParentClusterName: cc.Spec.ParentClusterName,
			Command:           command,
		},
	}
	switch command {
	case clustermessage.CommandType_ControlReq:
		task := clusterControllerCRDToSerializedControllerTask(cc)
		if task != nil {
			ret.Body = task
		}
	default:
		klog.Errorf("cluster controller crd command %s is not supported", command.String())
	}
	return ret
}

func clusterControllerCRDToSerializedControllerTask(
	cc *otev1.ClusterController) []byte {
	if cc == nil {
		return nil
	}
	ret := &clustermessage.ControllerTask{
		Destination: cc.Spec.Destination,
		Method:      cc.Spec.Method,
		URI:         cc.Spec.URL,
		Body:        []byte(cc.Spec.Body),
	}
	data, err := proto.Marshal(ret)
	if err != nil {
		klog.Errorf("marshal controller task failed: %v", err)
		return nil
	}
	return data
}

func clusterMessageToClusterControllerCRD(
	msg *clustermessage.ClusterMessage) *otev1.ClusterController {
	if msg == nil {
		return nil
	}
	ret := &otev1.ClusterController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msg.Head.MessageID,
			Namespace: otev1.ClusterNamespace,
		},
		Status: make(map[string]otev1.ClusterControllerStatus),
	}
	switch msg.Head.Command {
	case clustermessage.CommandType_ControlResp:
		cluster, status := clusterMessageToClusterControllerStatusCRD(msg)
		ret.Status[cluster] = *status
	default:
		klog.Errorf("command %s is not supported when transfer to crd", msg.Head.Command.String())
	}
	return ret
}

func clusterMessageToClusterControllerStatusCRD(
	msg *clustermessage.ClusterMessage) (string, *otev1.ClusterControllerStatus) {
	if msg == nil {
		return "", nil
	}
	controllerTaskResp := &clustermessage.ControllerTaskResponse{}
	err := proto.Unmarshal([]byte(msg.Body), controllerTaskResp)
	if err != nil {
		klog.Errorf("unmarshal controller task resp failed: %v", msg.Body)
		return "", nil
	}
	return msg.Head.ClusterName, &otev1.ClusterControllerStatus{
		Timestamp:  controllerTaskResp.Timestamp,
		StatusCode: int(controllerTaskResp.StatusCode),
		Body:       string(controllerTaskResp.Body),
	}
}
