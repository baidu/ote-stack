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
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetes "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/baidu/ote-stack/pkg/reporter"
)

var (
	serviceGroup = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
)

func newServiceGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(serviceGroup, "", name)
}

func newServiceCreateAction(service *corev1.Service) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(serviceGroup, "", service)
}

func newServiceUpdateAction(service *corev1.Service) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(serviceGroup, "", service)
}

func newServiceDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(serviceGroup, "", name)
}

func NewService(name string,
	clusterLabel string, edgeVersion string, resourceVersion string) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Labels:          map[string]string{reporter.ClusterLabel: clusterLabel, reporter.EdgeVersionLabel: edgeVersion},
			ResourceVersion: resourceVersion,
		},
	}

	return service
}

func TestHandleServiceReport(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	serviceUpdateMap := &reporter.ServiceResourceStatus{
		UpdateMap: make(map[string]*corev1.Service),
		DelMap:    make(map[string]*corev1.Service),
	}

	service := NewService("test-name1", "cluster1", "", "1")

	serviceUpdateMap.UpdateMap["test-namespace1/test-name1"] = service
	serviceUpdateMap.DelMap["test-namespace1/test-name2"] = service

	serviceJson, err := json.Marshal(serviceUpdateMap)
	assert.Nil(t, err)

	serviceReport := reporter.Report{
		ResourceType: reporter.ResourceTypeService,
		Body:         serviceJson,
	}

	reportJson, err := json.Marshal(serviceReport)
	assert.Nil(t, err)

	err = u.handleServiceReport(reportJson)
	assert.Nil(t, err)

	err = u.handleServiceReport([]byte{1})
	assert.NotNil(t, err)
}

func TestRetryServiceUpdate(t *testing.T) {
	u := NewUpstreamProcessor(&K8sContext{})

	service := NewService("test-name1", "cluster1", "11", "1")
	getService := NewService("test-name1", "cluster1", "1", "4")

	mockClient, tracker := newSimpleClientset(getService)

	// mock api server ResourceVersion conflict
	mockClient.PrependReactor("update", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
		etcdService := NewService("test-name1", "cluster1", "0", "9")

		if updateService, ok := action.(kubetesting.UpdateActionImpl); ok {
			if services, ok := updateService.Object.(*corev1.Service); ok {
				// ResourceVersion same length, can be compared with string
				if strings.Compare(etcdService.ResourceVersion, services.ResourceVersion) != 0 {
					err := tracker.Update(serviceGroup, etcdService, "")
					assert.Nil(t, err)
					return true, nil, kubeerrors.NewConflict(schema.GroupResource{}, "", nil)
				}
			}
		}
		return true, etcdService, nil
	})

	u.ctx.K8sClient = mockClient
	err := u.UpdateService(service)
	assert.Nil(t, err)
}

func TestGetCreateOrUpdateService(t *testing.T) {
	service1 := NewService("test1", "", "11", "10")
	service2 := NewService("test1", "", "12", "10")

	tests := []struct {
		name             string
		service          *corev1.Service
		getServiceResult *corev1.Service
		errorOnGet       error
		errorOnCreation  error
		errorOnUpdate    error
		expectActions    []kubetesting.Action
		expectErr        bool
	}{
		{
			name:       "Success to create a new service.",
			service:    service1,
			errorOnGet: kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			expectActions: []kubetesting.Action{
				newServiceGetAction(service1.Name),
				newServiceCreateAction(service1),
			},
			expectErr: false,
		},
		{
			name:            "A error occurs when create a new service fails.",
			service:         service1,
			errorOnGet:      kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreation: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newServiceGetAction(service1.Name),
				newServiceCreateAction(service1),
			},
			expectErr: true,
		},
		{
			name:             "A error occurs when create an existent service.",
			service:          service2,
			getServiceResult: service1,
			errorOnUpdate:    errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newServiceGetAction(service1.Name),
				newServiceUpdateAction(service1),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("get", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, test.getServiceResult, test.errorOnGet
			})
			mockClient.AddReactor("create", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnCreation
			})
			mockClient.AddReactor("update", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnUpdate
			})

			u.ctx.K8sClient = mockClient
			err := u.CreateOrUpdateService(test.service)

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

func TestDeleteService(t *testing.T) {
	testService := NewService("test1", "", "11", "10")

	tests := []struct {
		name             string
		service          *corev1.Service
		getServiceResult *corev1.Service
		errorOnGet       error
		errorOnDelete    error
		expectActions    []kubetesting.Action
		expectErr        bool
	}{
		{
			name:    "Success to delete an existent service.",
			service: testService,
			expectActions: []kubetesting.Action{
				newServiceDeleteAction(testService.Name),
			},
			expectErr: false,
		},
		{
			name:          "A error occurs when delete a service fails.",
			service:       testService,
			errorOnDelete: errors.New("wanted error"),
			expectActions: []kubetesting.Action{
				newServiceDeleteAction(testService.Name),
			},
			expectErr: true,
		},
	}

	u := NewUpstreamProcessor(&K8sContext{})

	for _, test := range tests {
		t.Run(test.name, func(t1 *testing.T) {
			assert := assert.New(t1)

			mockClient := &kubernetes.Clientset{}
			mockClient.AddReactor("delete", "services", func(action kubetesting.Action) (bool, runtime.Object, error) {
				return true, nil, test.errorOnDelete
			})

			u.ctx.K8sClient = mockClient
			err := u.DeleteService(test.service)

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
