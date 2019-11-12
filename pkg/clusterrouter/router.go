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

	"github.com/baidu/ote-stack/pkg/clustermessage"
	"github.com/baidu/ote-stack/pkg/config"
)

var (
	defaultClusterRouter = ClusterRouter{
		Childs:        make(map[string]string),
		subtreeRouter: make(map[string]string),
		rwMutex:       &sync.RWMutex{},
	}
)

// SubTreeRouter is a router which represents from a certain port to a certain node in subtree.
// The key is the cluster name of the node in subtree,
// and the value is the cluster name of the node directly connect to current node.
// A key-value pair means, in current node, you can reach to "key" from "value".
type SubTreeRouter map[string]string

// Serialize serializes the SubTreeRouter so as to send to neighbors.
func (s *SubTreeRouter) Serialize() ([]byte, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ClusterRouter consists of all router info of a node.
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

// Serialize serializes a ClusterRouter so as to send to neighbors.
func (cr *ClusterRouter) Serialize() ([]byte, error) {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	b, err := json.Marshal(cr)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Deserialize deserializes a ClusterRouter so as to synchronize with neighbors.
func (cr *ClusterRouter) Deserialize(b []byte) error {
	return json.Unmarshal(b, cr)
}

// RouterNotifier is a func to notify childs(...) of route info of current cluster.
type RouterNotifier func(*clustermessage.ClusterMessage, ...string)

// Router returns the default cluster router.
func Router() *ClusterRouter {
	return &defaultClusterRouter
}

// AddChild add a child named clusterName with listen addr,
// and call notifier if add successful.
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

// DelChild delete a chiled named clusterName,
// and call notifier if delete successful.
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

// HasChild returns if the current node has a child named clusterName.
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
	} else if port != oldPort {
		// there is a same name child to a diffrent port, refuse to add
		klog.Errorf(
			"route to %s already exist from port %s, add route %s-%s failed",
			to, oldPort, to, port)
		return config.ErrDuplicatedName
	}
	klog.Infof("route update: %v", cr.subtreeRouter)
	return nil
}

/*
DelRoute delete a route.
to is cluster name of node in subtree,
port is cluster name of a child which can reach to node.
*/
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

// HasRoute returns if the current node has a route from "port" to "to".
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
	ret := make([]string, 0)
	for to, p := range cr.subtreeRouter {
		if p == port && to != port {
			ret = append(ret, to)
		}
	}
	return ret
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

// ParentNeighbors return neighbors of parent cluster.
// key is cluster name, and value is listen address of the cluster.
func (cr *ClusterRouter) ParentNeighbors() map[string]string {
	return cr.ParentNeighbor
}

// NeighborRouterMessage wrap router info to cluster message.
func (cr *ClusterRouter) NeighborRouterMessage() *clustermessage.ClusterMessage {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()
	cbyte, err := cr.Serialize()
	if err != nil {
		klog.Errorf("serialize cluster router %v failed: %v", cr, err)
		return nil
	}
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_NeighborRoute,
		},
		Body: cbyte,
	}
	return msg
}

// SubTreeMessage wrap subtree router info to cluster message.
func (cr *ClusterRouter) SubTreeMessage() *clustermessage.ClusterMessage {
	cr.rwMutex.RLock()
	defer cr.rwMutex.RUnlock()

	cbyte, err := cr.subtreeRouter.Serialize()
	if err != nil {
		klog.Errorf("serialize subtree router %v failed: %v", cr, err)
		return nil
	}
	msg := &clustermessage.ClusterMessage{
		Head: &clustermessage.MessageHead{
			Command: clustermessage.CommandType_SubTreeRoute,
		},
		Body: cbyte,
	}
	return msg
}

// SubtreeFromClusterController get subtree router info from a cluster message.
func SubtreeFromClusterController(msg *clustermessage.ClusterMessage) SubTreeRouter {
	ret := SubTreeRouter{}
	err := json.Unmarshal(msg.Body, &ret)
	if err != nil {
		klog.Errorf("deserialize cluster subtree failed: %v", err)
		return nil
	}
	return ret
}

func neighborRouterFromClusterMessage(msg *clustermessage.ClusterMessage) *ClusterRouter {
	ret := ClusterRouter{}
	err := ret.Deserialize(msg.Body)
	if err != nil {
		klog.Errorf("deserialize cluster neighbor router failed: %v", err)
		return nil
	}
	return &ret
}

// UpdateRouter updates router of current cluster and notify child.
func UpdateRouter(msg *clustermessage.ClusterMessage, notifier RouterNotifier) {
	r := neighborRouterFromClusterMessage(msg)
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
