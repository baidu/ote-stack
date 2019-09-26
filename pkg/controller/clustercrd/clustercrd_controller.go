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

//Package clustercrd watch cluster crd if it is created,
//get all namespace from center etcd,and send them to the new cluster.
package clustercrd

import (
	"encoding/json"
	"fmt"

	"github.com/golang/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/controllermanager"
	oteinformer "github.com/baidu/ote-stack/pkg/generated/informers/externalversions/ote/v1"
)

const (
	oteNamespaceKind = "Namespace"
	oteNamespaceURI  = "/api/v1/namespaces"
	oteApiVersionV1  = "v1"
)

//ClusterCrdController is responsible for performing actions dependent upon a cluster phase.
type ClusterCrdController struct {
	sendChan  chan clustermessage.ClusterMessage
	k8sClient kubernetes.Interface
}

//oteNamespace is responsible for constructing namespace object.
type oteNamespace struct {
	Kind       string            `json:"kind"`
	ApiVersion string            `json:"apiVersion"`
	MetaData   map[string]string `json:"metadata"`
}

//NewClusterCrdController creates a new ClusterCrdController.
func NewClusterCrdController(
	clusterInformer oteinformer.ClusterInformer,
	kubeClient kubernetes.Interface,
	publishChan chan clustermessage.ClusterMessage) *ClusterCrdController {

	clusterCrdController := &ClusterCrdController{
		sendChan:  publishChan,
		k8sClient: kubeClient,
	}

	clusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: clusterCrdController.HandleAddedEvent,
	})

	return clusterCrdController
}

//InitClusterCrdController inits clustercrd controller.
func InitClusterCrdController(ctx *controllermanager.ControllerContext) error {
	NewClusterCrdController(
		ctx.OteInformerFactory.Ote().V1().Clusters(),
		ctx.K8sClient, ctx.PublishChan)
	return nil
}

//HandleAddedEvent handles Added Event when new cluster was created.
//There are things to do when new cluster was created:
//1.send namespace to new cluster.
//2.TODO send other resource to new cluster.
func (c *ClusterCrdController) HandleAddedEvent(obj interface{}) {
	cluster := obj.(*otev1.Cluster)
	klog.Infof("new cluster added: %v", cluster.ObjectMeta.Name)

	err := c.SendNamespaceToNewCluster(cluster)
	if err != nil {
		klog.Errorf("send namespace to cluster %v failed: %v", cluster.ObjectMeta.Name, err)
	}
}

//SendNamespaceToNewCluster gets all namespace from center etcd
//and sends them to the new cluster.
func (c *ClusterCrdController) SendNamespaceToNewCluster(cluster *otev1.Cluster) error {
	var body [][]byte

	nameList, err := c.GetNamespaceList()
	if err != nil {
		return fmt.Errorf("get NamespaceList failed: %v", err)
	}
	for _, item := range nameList {
		name, err := SerializeNamespaceObject(item)
		if err != nil {
			klog.Errorf("serialize namespace object %v failed: %v", item, err)
			continue
		}
		body = append(body, name)
	}

	data := &clustermessage.ControlMultiTask{
		Destination: otev1.ClusterControllerDestAPI,
		Method:      "POST",
		URI:         oteNamespaceURI,
		Body:        body,
	}
	msg := ControlMultiTaskToClusterMessage(cluster.Spec.Name, data)
	if msg == nil {
		return fmt.Errorf("ControlMultiTask to ClusterMessage failed.")
	}

	c.sendChan <- *msg
	return nil
}

//getNamespaceList gets NamespaceList from center etcd.
func (c *ClusterCrdController) GetNamespaceList() ([]string, error) {
	var ret []string

	nsList, err := c.k8sClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, item := range nsList.Items {
		ret = append(ret, item.Name)
	}
	return ret, nil
}

//SerializeNamespaceObject serializes an oteNamespace to be the body of
//k8s rest request with specific name.
func SerializeNamespaceObject(name string) ([]byte, error) {
	data := make(map[string]string)
	data["name"] = name
	msg := oteNamespace{
		Kind:       oteNamespaceKind,
		ApiVersion: oteApiVersionV1,
		MetaData:   data,
	}
	ret, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal Namespace Object failed.")
	}
	return ret, nil
}

//ControlMultiTaskToClusterMessage makes ControlMultiTask to ClusterMessage.
func ControlMultiTaskToClusterMessage(name string,
	msg *clustermessage.ControlMultiTask) *clustermessage.ClusterMessage {
	if name == "" || msg == nil {
		return nil
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		klog.Errorf("marshal ControlMultiTask failed: %v", err)
		return nil
	}

	ret := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command:         clustermessage.CommandType_ControlMultiReq,
			ClusterSelector: name,
		},
		Body: data,
	}
	return ret
}
