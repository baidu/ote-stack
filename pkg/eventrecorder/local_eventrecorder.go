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

// Package eventrecorder defines event recorder for k8s leader election.
package eventrecorder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
)

// LocalEventRecorder is a recorder for leaderelection which print event log to local logs.
// LocalEventRecorder implements recode.EventRecorder interface.
// only Eventf func is needed.
type LocalEventRecorder struct{}

func (ler *LocalEventRecorder) Eventf(obj runtime.Object, eventType, reason, message string, args ...interface{}) {
	klog.Infof("local event record[%v][%s][%s][%s]%v",
		obj, eventType, reason, message, args)
}

func (ler *LocalEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {}

func (ler *LocalEventRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
}

func (ler *LocalEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}
