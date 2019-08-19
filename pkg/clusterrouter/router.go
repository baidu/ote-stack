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

// Package clusterrouter manages cluster route of subtree and neighbor.
/*
Route information keeps my childs, neighbor and parent neighbor.

There are 2 ways to receive a route message:

1. cloud-tunnel: must be a child-connect/disconnect message.

Once get one, route should be updated,
besides, notify all other childs, so they can update their neighbor.

2. edge-tunnel(EdgeToClusterChan): my neighbor or parent neighbor may changed.

If my parent neighbor changed, update my route,
if my neighbor changed, notify my childs in addition.
*/
package clusterrouter

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"k8s.io/klog"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	"github.com/baidu/ote-stack/pkg/config"
)

var (
	defaultClusterRouter = ClusterRouter{
		Childs:        make(map[string]string),
		subtreeRouter: make(map[string]string),
		rwMutex:       &sync.RWMutex{},
	}
)

type SubTreeRouter map[string]string

func (s *SubTreeRouter) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

type ClusterRouter struct {
	Childs         map[string]string // cluster name -> cluster tunnel listen address
	Neighbor       map[string]string // same as above
	ParentNeighbor map[string]string // same as above
	// key is cluster name of node in subtree
	// subtreeRouter should not serialized to json string to send to childs or parent
	// value should be string if cluster name is universally unique
	subtreeRouter SubTreeRouter

	rwMutex *sync.RWMutex
}

func (cr *ClusterRouter) Serialize() ([]byte, error) {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	b, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (cr *ClusterRouter) Deserialize(b []byte) error {
	return json.Unmarshal(b, cr)
}

// RouterNotifier is a func to notify childs(...) of route info of current cluster.
type RouterNotifier func(*otev1.ClusterController, ...string)

// Router returns the default cluster router.
func Router() *ClusterRouter {
	return &defaultClusterRouter
}

func (cr *ClusterRouter) AddChild(clusterName, listen string, notifier RouterNotifier) error {
	err := func() error {
		cr.rwMutex.Lock()
		defer cr.rwMutex.Unlock()

		if ip, ok := cr.Childs[clusterName]; ok {
			// already has a child with same name
			return fmt.Errorf("%s has been used at %s, refuse to add this one at %s", clusterName, ip, listen)
		}
		cr.Childs[clusterName] = listen
		klog.V(3).Infof("add child(%s-%s) to route", clusterName, listen)
		klog.Infof("cluster neighbor router updated: %#v", defaultClusterRouter)
		return nil
	}()
	if err != nil {
		return err
	}

	notifier(defaultClusterRouter.NeighborRouterMessage())

	return nil
}

func (cr *ClusterRouter) DelChild(clusterName string, notifier RouterNotifier) {
	func() {
		cr.rwMutex.Lock()
		defer cr.rwMutex.Unlock()

		if _, ok := cr.Childs[clusterName]; !ok {
			klog.Errorf("no child named %s, cannot delete", clusterName)
			return
		}
		delete(cr.Childs, clusterName)
		klog.V(3).Infof("del child(%s) from route", clusterName)
		klog.Infof("cluster neighbor router updated: %#v", defaultClusterRouter)
	}()

	notifier(defaultClusterRouter.NeighborRouterMessage())
}

func (cr *ClusterRouter) HasChild(clusterName string) bool {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	_, ok := cr.Childs[clusterName]
	return ok
}

/*
AddRoute add a route.
to is cluster name of node in subtree,
port is cluster name of a child which can reach to node.
*/
func (cr *ClusterRouter) AddRoute(to, port string) error {
	cr.rwMutex.Lock()
	defer cr.rwMutex.Unlock()

	if oldPort, ok := cr.subtreeRouter[to]; !ok {
		cr.subtreeRouter[to] = port
	} else {
		// there is a same name child, refuse to add
		klog.Errorf(
			"route to %s already exist from port %s, add route %s-%s failed",
			to, oldPort, to, port)
		return config.ErrDuplicatedName
	}
	klog.Infof("route update: %v", cr.subtreeRouter)
	return nil
}

func (cr *ClusterRouter) DelRoute(to, port string) {
	cr.rwMutex.Lock()
	defer cr.rwMutex.Unlock()

	if oldPort, ok := cr.subtreeRouter[to]; ok {
		if oldPort == port {
			delete(cr.subtreeRouter, to)
		} else {
			klog.Errorf("port is different, delete route failed. old: %s, ask: %s", oldPort, port)
		}
	}
	if to == port {
		// if it is a route to child need to remove
		// delete route from port
		delete(cr.subtreeRouter, to)
		for key, oldPort := range cr.subtreeRouter {
			if oldPort == port {
				delete(cr.subtreeRouter, key)
			}
		}
	}

	klog.Infof("route update: %v", cr.subtreeRouter)
}

func (cr *ClusterRouter) HasRoute(to, port string) bool {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	if oldPort, ok := cr.subtreeRouter[to]; ok && oldPort == port {
		return true
	}
	return false
}

/*
PortsToSubtreeClusters get ports which can reach to clusters.
return is a map whose key is cluster name of a port,
value is subtree names of port.
*/
func (cr *ClusterRouter) PortsToSubtreeClusters(clusters *[]string) map[string][]string {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	// TODO remove duplicated clusters
	ret := make(map[string][]string)
	for _, c := range *clusters {
		if port, ok := cr.subtreeRouter[c]; ok {
			if subs, ok := ret[port]; ok {
				ret[port] = append(subs, c)
			} else {
				subs = make([]string, 1)
				subs[0] = c
				ret[port] = subs
			}
		}
	}
	klog.V(3).Infof("select PortsToSubtreeClusters: %v, %v", clusters, ret)
	return ret
}

// SubTreeOfPort return a slice of cluster names which is in the subtree under a certain port.
func (cr *ClusterRouter) SubTreeOfPort(port string) []string {
	// TODO
	return nil
}

// SubTreeClusters return all cluster names under current cluster.
func (cr *ClusterRouter) SubTreeClusters() []string {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	ret := make([]string, len(cr.subtreeRouter))
	count := 0
	for key := range cr.subtreeRouter {
		ret[count] = key
		count++
	}
	return ret
}

// updateNeighbor update neighbor of current cluster.
// return true if neighbor changed, return false otherwise.
func (cr *ClusterRouter) updateNeighbor(parentRouter *ClusterRouter) bool {
	if reflect.DeepEqual(cr.Neighbor, parentRouter.Childs) {
		return false
	}
	cr.Neighbor = parentRouter.Childs
	return true
}

// updateParentNeighbor update parent neighbor of current cluster.
// return true if parent neighbor changed, return false otherwise
func (cr *ClusterRouter) updateParentNeighbor(parentRouter *ClusterRouter) bool {
	if reflect.DeepEqual(cr.ParentNeighbor, parentRouter.Neighbor) {
		return false
	}
	cr.ParentNeighbor = parentRouter.Neighbor
	return true
}

func (cr *ClusterRouter) ParentNeighbors() map[string]string {
	return cr.ParentNeighbor
}

func (cr *ClusterRouter) NeighborRouterMessage() *otev1.ClusterController {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()
	cbyte, err := cr.Serialize()
	if err != nil {
		klog.Errorf("serialize cluster router %v failed: %v", cr, err)
		return nil
	}
	cc := otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			Destination: otev1.ClusterControllerDestClusterRoute,
			Body:        string(cbyte),
		},
	}
	return &cc
}

func (cr *ClusterRouter) SubTreeMessage() *otev1.ClusterController {
	if len(cr.subtreeRouter) == 0 {
		return nil
	}

	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	cbyte, err := cr.subtreeRouter.Serialize()
	if err != nil {
		klog.Errorf("serialize subtree router %v failed: %v", cr, err)
		return nil
	}
	cc := otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			Destination: otev1.ClusterControllerDestClusterSubtree,
			Body:        string(cbyte),
		},
	}
	return &cc
}

func SubtreeFromClusterController(cc *otev1.ClusterController) SubTreeRouter {
	ret := SubTreeRouter{}
	err := json.Unmarshal([]byte(cc.Spec.Body), &ret)
	if err != nil {
		klog.Errorf("deserialize cluster subtree failed: %v", err)
		return nil
	}
	return ret
}

func neighborRouterFromClusterController(cc *otev1.ClusterController) *ClusterRouter {
	ret := ClusterRouter{}
	err := ret.Deserialize([]byte(cc.Spec.Body))
	if err != nil {
		klog.Errorf("deserialize cluster neighbor router failed: %v", err)
		return nil
	}
	return &ret
}

// UpdateRouter updates router of current cluster and notify child.
func UpdateRouter(cc *otev1.ClusterController, notifier RouterNotifier) {
	r := neighborRouterFromClusterController(cc)
	if r == nil {
		return
	}
	// r is route of parent
	if defaultClusterRouter.updateNeighbor(r) {
		notifier(defaultClusterRouter.NeighborRouterMessage())
	}
	defaultClusterRouter.updateParentNeighbor(r)
	klog.Infof("cluster router updated: %#v", defaultClusterRouter)
}
