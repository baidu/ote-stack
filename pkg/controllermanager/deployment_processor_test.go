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

package controllermanager

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	deploymentGroup = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deploymentKind  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
)

func newDeploymentGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(deploymentGroup, "", name)
}

func newDeploymentCreateAction(deployment *appsv1.Deployment) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(deploymentGroup, "", deployment)
}

func newDeploymentUpdateAction(deployment *appsv1.Deployment) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(deploymentGroup, "", deployment)
}

func newDeploymentDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(deploymentGroup, "", name)
}

func newDeploymentListAction(ops metav1.ListOptions) kubetesting.ListActionImpl {
	return kubetesting.NewListAction(deploymentGroup, deploymentKind, "", ops)
}

func NewDeployment(name string,
	clusterLabel string, edgeVersion string, resourceVersion string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          map[string]string{reporter.ClusterLabel: clusterLabel, reporter.EdgeVersionLabel: edgeVersion},
			ResourceVersion: resourceVersion,
		},
	}

	return deployment
}

func TestHandleDeploymentReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})
	u.ctx.K8sClient = &kubernetes.Clientset{}

	deploymentUpdateMap := &reporter.DeploymentResourceStatus{
		UpdateMap: make(map[string]*appsv1.Deployment),
		DelMap:    make(map[string]*appsv1.Deployment),
		FullList:  make([]string, 1),
	}

	deployment := NewDeployment("test-name1", "cluster1", "", "1")

	deploymentUpdateMap.UpdateMap["test-namespace1/test-name1"] = deployment
	deploymentUpdateMap.DelMap["test-namespace1/test-name2"] = deployment
	deploymentUpdateMap.FullList = []string{"dp1"}

	deploymentJson, err := json.Marshal(deploymentUpdateMap)
	assert.Nil(t, err)

	err = u.handleDeploymentReport("cluster1", deploymentJson)
	assert.Nil(t, err)

	err = u.handleDeploymentReport("cluster1", []byte{1})
	assert.NotNil(t, err)
}

func TestRetryDeploymentUpdate(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	deployment := NewDeployment("test-name1", "cluster1", "11", "1")
	getDeployment := NewDeployment("test-name1", "cluster1", "1", "4")

	mockClient, tracker := newSimpleClientset(getDeployment)

	// mock api server ResourceVersion conflict
	mockClient.PrependReactor("update", "deployments", func(action kubetesting.Action) (bool, runtime.Object, error) {
		etcdDeployment := NewDeployment("test-name1", "cluster1", "0", "9")

		if updateDeployment, ok := action.(kubetesting.UpdateActionImpl); ok {
			if deployments, ok := updateDeployment.Object.(*appsv1.Deployment); ok {
				// ResourceVersion same length, can be compared with string
				if strings.Compare(etcdDeployment.ResourceVersion, deployments.ResourceVersion) != 0 {
					err := tracker.Update(deploymentGroup, etcdDeployment, "")
					assert.Nil(t, err)
					return true, nil, kubeerrors.NewConflict(schema.GroupResource{}, "", nil)
				}
			}
		}
		return true, etcdDeployment, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.UpdateDeployment(deployment)
	assert.Nil(t, err)
}

func TestGetCreateOrUpdateDeployment(t *testing.T) {
	deployment1 := NewDeployment("test1", "", "11", "10")

	tests := []struct {
		name                string
		deployment          *appsv1.Deployment
		getDeploymentResult *appsv1.Deployment
		errorOnGet          error
		errorOnCreation     error
		errorOnUpdate       error
		expectActions       []kubetesting.Action
		expectErr           bool
	}{
		{
			name:       "A error occurs when create a deployment.",
			deployment: deployment1,
			errorOnGet: kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			expectActions: []kubetesting.Action{
				newDeploymentGetAction(deployment1.Name),
				newDeploymentCreateAction(deployment1),
				newDeploymentGetAction(deployment1.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("get", "deployments", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, test.getDeploymentResult, test.errorOnGet
			})
			mockClient.AddReactor("create", "deployments", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateOrUpdateDeployment(test.deployment)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes.
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestDeleteDeployment(t *testing.T) {
	testDeployment := NewDeployment("test1", "", "11", "10")

	tests := []struct {
		name                string
		deployment          *appsv1.Deployment
		getDeploymentResult *appsv1.Deployment
		errorOnGet          error
		errorOnDelete       error
		expectActions       []kubetesting.Action
		expectErr           bool
	}{
		{
			name:       "Success to delete an existent deployment.",
			deployment: testDeployment,
			expectActions: []kubetesting.Action{
				newDeploymentDeleteAction(testDeployment.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a deployment fails.",
			deployment:    testDeployment,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newDeploymentDeleteAction(testDeployment.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("delete", "deployments", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeleteDeployment(test.deployment)

			if test.expectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
				// Check calls to kubernetes.
				assert.Equal(test.expectActions, mockClient.Actions())
			}
		})
	}
}

func TestHandleDeploymentFullList(t *testing.T) {
	fullList := []string{"dp1"}

	ops := metav1.ListOptions{
		LabelSelector: "ote-cluster=c1",
	}

	cmDeploymentList := &appsv1.DeploymentList{}
	deployment := NewDeployment("dp1-c1", "c1", "", "")
	cmDeploymentList.Items = append(cmDeploymentList.Items, *deployment)

	cmDeploymentList2 := &appsv1.DeploymentList{}
	deployment = NewDeployment("dp2-c1", "c1", "", "")
	cmDeploymentList2.Items = append(cmDeploymentList2.Items, *deployment)

	tests := []struct {
		name               string
		clusterName        string
		edgeDeploymentList []string
		deploymentList     *appsv1.DeploymentList
		expectActions      []kubetesting.Action
		expectErr          bool
	}{
		{
			name:               "success to handle full list's deployment",
			clusterName:        "c1",
			edgeDeploymentList: fullList,
			deploymentList:     cmDeploymentList,
			expectActions: []kubetesting.Action{
				newDeploymentListAction(ops),
			},
			expectErr: false,
		},
		{
			name:               "A error occurs when handles a full list deployment",
			clusterName:        "c1",
			edgeDeploymentList: fullList,
			deploymentList:     cmDeploymentList2,
			expectActions: []kubetesting.Action{
				newDeploymentListAction(ops),
				newDeploymentDeleteAction("dp2"),
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		mockClient := &kubernetes.Clientset{}
		mockClient.AddReactor("list", "deployments", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.deploymentList, nil
		})

		u := NewUpstreamProcessor(&K8sContext{})
		u.ctx.K8sClient = mockClient

		u.handleDeploymentFullList(test.clusterName, test.edgeDeploymentList)

		if test.expectErr {
			assert.NotEqual(t, test.expectActions, mockClient.Actions())
		} else {
			assert.Equal(t, test.expectActions, mockClient.Actions())
		}
	}
}
