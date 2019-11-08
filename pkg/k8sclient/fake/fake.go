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

// Package fake mocks clientset for ote.
package fake

import (
	"encoding/json"
	"fmt"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubetesting "k8s.io/client-go/testing"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	oteclient "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

var (
	scheme             = runtime.NewScheme()
	codecs             = serializer.NewCodecFactory(scheme)
	parameterCodec     = runtime.NewParameterCodec(scheme)
	localSchemeBuilder = runtime.SchemeBuilder{
		otev1.AddToScheme,
	}
	addToScheme = localSchemeBuilder.AddToScheme
)

func init() {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	utilruntime.Must(addToScheme(scheme))
}

// NewSimpleClientset mock a clientset for cluster crd since
// client-go v10 version do not support merge json patch.
func NewSimpleClientset(objects ...runtime.Object) *oteclient.Clientset {
	o := kubetesting.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &oteclient.Clientset{}
	cs.AddReactor("patch", "clusters", func(action kubetesting.Action) (bool, runtime.Object, error) {
		ns := action.GetNamespace()
		gvr := action.GetResource()
		switch action := action.(type) {
		case kubetesting.PatchActionImpl:
			obj, err := o.Get(gvr, ns, action.GetName())
			if err != nil {
				return true, nil, err
			}

			old, err := json.Marshal(obj)
			if err != nil {
				return true, nil, err
			}

			value := reflect.ValueOf(obj)
			value.Elem().Set(reflect.New(value.Type().Elem()).Elem())
			modified, err := jsonpatch.MergePatch(old, action.GetPatch())
			if err != nil {
				return true, nil, err
			}

			if err := json.Unmarshal(modified, obj); err != nil {
				return true, nil, err
			}
			if err = o.Update(gvr, obj, ns); err != nil {
				return true, nil, err
			}

			return true, obj, nil
		default:
			return false, nil, fmt.Errorf("no reaction implemented for %s", action)

		}
	})

	cs.AddReactor("*", "*", kubetesting.ObjectReaction(o))
	cs.AddWatchReactor("*", func(action kubetesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := o.Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		return true, watch, nil
	})

	return cs
}
