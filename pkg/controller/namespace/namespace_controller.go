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

//Package namespace watch Namespace resource if it is created,
//and send it to all clusters.
package namespace

import (
	"fmt"
	"net/http"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/controller"
	"github.com/baidu/ote-stack/pkg/controllermanager"
)

//NamespaceController is responsible for performing actions dependent upon a namespace phase.
type NamespaceController struct {
	sendChan chan clustermessage.ClusterMessage
}

//InitNamespaceController inits namespace controller.
func InitNamespaceController(ctx *controllermanager.ControllerContext) error {
	namespaceController := &NamespaceController{
		sendChan: ctx.PublishChan,
	}
	ctx.InformerFactory.Core().V1().Namespaces().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: namespaceController.handleAddedEvent,
	})

	return nil
}

//handleAddedEvent handles Added Event when new namespace was created,
//and send the new namespace to all clusters.
func (c *NamespaceController) handleAddedEvent(obj interface{}) {
	namespace := obj.(*v1.Namespace)
	klog.V(3).Infof("new namespace added: %v", namespace.ObjectMeta.Name)

	err := c.sendNamespaceToCluster(namespace)
	if err != nil {
		klog.Errorf("send namespace to cluster failed: %v", err)
	}
}

//sendNamespaceToCluster sends new namespace to all clusters.
func (c *NamespaceController) sendNamespaceToCluster(namespace *v1.Namespace) error {
	name, err := controller.SerializeNamespaceObject(namespace.ObjectMeta.Name)
	if err != nil {
		return fmt.Errorf("serialize namespace object %s failed: %v", namespace.ObjectMeta.Name, err)
	}

	data := &clustermessage.ControllerTask{
		Destination: otev1.ClusterControllerDestAPI,
		Method:      http.MethodPost,
		URI:         controller.OteNamespaceURI,
		Body:        name,
	}
	head := &clustermessage.MessageHead{
		ClusterSelector: "",
		Command:         clustermessage.CommandType_ControlReq,
	}
	msg, err := data.ToClusterMessage(head)
	if err != nil {
		return err
	}

	c.sendChan <- *msg
	return nil
}
