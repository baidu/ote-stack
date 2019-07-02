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

// Pakcage clusterrouter manages cluster route of subtree and neighbor.
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
)

var (
	defaultClusterRouter = ClusterRouter{
		Childs:        make(map[string]string),
		subtreeRouter: make(map[string]map[string]int),
		rwMutex:       &sync.RWMutex{},
	}
)

// ClusterRouter is the interface to manipulate route of a cluster.
//type ClusterRouter interface {
//	Serialize() ([]byte, error)
//	Deserialize([]byte) error
//	AddChild(string, string, RouterNotifier) error
//	DelChild(string, RouterNotifier)
//	AddRoute(string, string)
//	DelRoute(string, string)
//	HasRoute(string, string) bool
//	RouterMessage() *otev1.ClusterController
//	PortsToSubtreeClusters(*[]string) *map[string][]string
//	SubTreeClusters() *[]string
//	ParentNeighbors() map[string]string
//}

type ClusterRouter struct {
	Childs         map[string]string // cluster name -> cluster tunnel listen address
	Neighbor       map[string]string // same as above
	ParentNeighbor map[string]string // same as above
	// key is cluster name of node in subtree
	// value is the port->count to send msg out so it can reach to key
	// subtreeRouter should not serialized to json string to send to childs or parent
	// TODO value should be string if cluster name is universally unique
	// at least now, cluster name may be duplicated, so define value as map[string]int
	subtreeRouter map[string]map[string]int

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
		klog.Infof("cluster router updated: %#v", defaultClusterRouter)
		return nil
	}()
	if err != nil {
		return err
	}

	notifier(defaultClusterRouter.RouterMessage())

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
		klog.Infof("cluster router updated: %#v", defaultClusterRouter)
	}()

	notifier(defaultClusterRouter.RouterMessage())
}

/*
AddRoute add a route.
to is cluster name of node in subtree,
port is cluster name of a child which can reach to node.
*/
func (cr *ClusterRouter) AddRoute(to, port string) {
	cr.rwMutex.Lock()
	defer cr.rwMutex.Unlock()

	if ports, ok := cr.subtreeRouter[to]; !ok {
		ports = make(map[string]int)
		ports[port] = 1
		cr.subtreeRouter[to] = ports
	} else {
		if count, ok := ports[port]; !ok {
			ports[port] = 1
		} else {
			ports[port] = count + 1
		}
	}
	klog.V(3).Infof("route update: %v", cr.subtreeRouter)
}

func (cr *ClusterRouter) DelRoute(to, port string) {
	cr.rwMutex.Lock()
	defer cr.rwMutex.Unlock()

	if ports, ok := cr.subtreeRouter[to]; !ok {
		return
	} else {
		if count, ok := ports[port]; !ok {
			return
		} else {
			ports[port] = count - 1
			if ports[port] <= 0 {
				delete(ports, port)
			}
			if len(ports) == 0 {
				delete(cr.subtreeRouter, to)
			}
		}
	}
	if to == port {
		// if it is a route to child need to remove
		// delete route from port
		delete(cr.subtreeRouter, to)
		for key, ports := range cr.subtreeRouter {
			delete(ports, port)
			if len(ports) == 0 {
				delete(cr.subtreeRouter, key)
			}

		}
	}

	klog.V(3).Infof("route update: %v", cr.subtreeRouter)
}

func (cr *ClusterRouter) HasRoute(to, port string) bool {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	if ports, ok := cr.subtreeRouter[to]; !ok {
		return false
	} else {
		if count, ok := ports[port]; !ok {
			return false
		} else {
			if count > 0 {
				return true
			} else {
				return false
			}
		}
	}
}

/*
PortsToSubtreeClusters get ports which can reach to clusters.
return is a map whose key is cluster name of a port,
value is subtree names of port.
*/
func (cr *ClusterRouter) PortsToSubtreeClusters(clusters *[]string) *map[string][]string {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	// TODO remove duplicated clusters
	ret := make(map[string][]string)
	for _, c := range *clusters {
		if ports, ok := cr.subtreeRouter[c]; ok {
			for port, _ := range ports {
				if subs, ok := ret[port]; ok {
					ret[port] = append(subs, c)
				} else {
					subs = make([]string, 1)
					subs[0] = c
					ret[port] = subs
				}
			}
		}
	}
	klog.V(3).Infof("PortsToSubtreeClusters: %v, %v", clusters, ret)
	return &ret
}

// SubTreeClusters return all cluster names under current cluster.
func (cr *ClusterRouter) SubTreeClusters() *[]string {
	ret := make([]string, len(cr.subtreeRouter))
	count := 0
	for key, _ := range cr.subtreeRouter {
		ret[count] = key
		count++
	}
	return &ret
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

func (cr *ClusterRouter) RouterMessage() *otev1.ClusterController {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()
	cbyte, err := cr.Serialize()
	if err != nil {
		klog.Errorf("serialize cluster router %v failed: %v", cr, err)
		return nil
	}
	cc := otev1.ClusterController{
		Spec: otev1.ClusterControllerSpec{
			Destination: otev1.CLUSTER_CONTROLLER_DEST_CLUSTER_ROUTE,
			Body:        string(cbyte),
		},
	}
	return &cc
}

func routerFromClusterController(cc *otev1.ClusterController) *ClusterRouter {
	ret := ClusterRouter{}
	err := ret.Deserialize([]byte(cc.Spec.Body))
	if err != nil {
		klog.Errorf("deserialize cluster router failed: %v", err)
		return nil
	}
	return &ret
}

// UpdateRouter updates router of current cluster and notify child.
func UpdateRouter(cc *otev1.ClusterController, notifier RouterNotifier) {
	r := routerFromClusterController(cc)
	if r == nil {
		return
	}
	// r is route of parent
	if defaultClusterRouter.updateNeighbor(r) {
		notifier(defaultClusterRouter.RouterMessage())
	}
	defaultClusterRouter.updateParentNeighbor(r)
	klog.Infof("cluster router updated: %#v", defaultClusterRouter)
}
