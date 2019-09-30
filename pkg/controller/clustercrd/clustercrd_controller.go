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
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/controller"
	"github.com/baidu/ote-stack/pkg/controllermanager"
)

//ClusterCrdController is responsible for performing actions dependent upon a cluster phase.
type ClusterCrdController struct {
	sendChan  chan clustermessage.ClusterMessage
	k8sClient kubernetes.Interface
}

//InitClusterCrdController inits clustercrd controller.
func InitClusterCrdController(ctx *controllermanager.ControllerContext) error {
	clusterCrdController := &ClusterCrdController{
		sendChan:  ctx.PublishChan,
		k8sClient: ctx.K8sClient,
	}

	ctx.OteInformerFactory.Ote().V1().Clusters().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: clusterCrdController.handleAddedEvent,
	})

	return nil
}

//handleAddedEvent handles Added Event when new cluster was created.
//There are things to do when new cluster was created:
//1.send namespace to new cluster.
//2.TODO send other resource to new cluster.
func (c *ClusterCrdController) handleAddedEvent(obj interface{}) {
	cluster := obj.(*otev1.Cluster)
	klog.V(3).Infof("new cluster added: %v", cluster.ObjectMeta.Name)

	err := c.sendNamespaceToNewCluster(cluster)
	if err != nil {
		klog.Errorf("send namespace to cluster %v failed: %v", cluster.ObjectMeta.Name, err)
	}
}

//sendNamespaceToNewCluster gets all namespace from center etcd
//and sends them to the new cluster.
func (c *ClusterCrdController) sendNamespaceToNewCluster(cluster *otev1.Cluster) error {
	var body [][]byte

	nameList, err := c.getNamespaceList()
	if err != nil {
		return fmt.Errorf("get NamespaceList failed: %v", err)
	}
	for _, item := range nameList {
		name, err := controller.SerializeNamespaceObject(item)
		if err != nil {
			klog.Errorf("serialize namespace object %s failed: %v", item, err)
			continue
		}
		body = append(body, name)
	}

	data := &clustermessage.ControlMultiTask{
		Destination: otev1.ClusterControllerDestAPI,
		Method:      http.MethodPost,
		URI:         controller.OteNamespaceURI,
		Body:        body,
	}
	head := &clustermessage.MessageHead{
		ClusterSelector: cluster.Spec.Name,
		Command:         clustermessage.CommandType_ControlMultiReq,
	}
	msg, err := data.ToClusterMessage(head)
	if err != nil {
		return err
	}

	c.sendChan <- *msg
	return nil
}

//getNamespaceList gets NamespaceList from center etcd.
func (c *ClusterCrdController) getNamespaceList() ([]string, error) {
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
