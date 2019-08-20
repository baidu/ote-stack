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

package clusterrouter

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/baidu/ote-stack/pkg/clustermessage"
)

var (
	calledNotifier = false
)

func TestChildAndNeighbor(t *testing.T) {
	r := Router()

	assert.NoError(t, r.AddChild("c1", "192.168.0.2:1234", testRouterNotifier))
	assert.Error(t, r.AddChild("c1", "", testRouterNotifier))
	assert.Contains(t, defaultClusterRouter.Childs, "c1")
	assert.True(t, r.HasChild("c1"))

	r.DelChild("c1", testRouterNotifier)
	assert.NotContains(t, defaultClusterRouter.Childs, "c1")
	assert.False(t, r.HasChild("c1"))
	r.DelChild("c1", testRouterNotifier)

	parentR := ClusterRouter{
		Childs:  map[string]string{"c2": ""},
		rwMutex: &sync.RWMutex{},
	}
	assert.NotContains(t, defaultClusterRouter.Neighbor, "c2")

	parentRouteMsg := parentR.NeighborRouterMessage()
	calledNotifier = false
	assert.False(t, calledNotifier)
	UpdateRouter(parentRouteMsg, testRouterNotifier)
	assert.True(t, calledNotifier)
	assert.Contains(t, defaultClusterRouter.Neighbor, "c2")
	calledNotifier = false
	UpdateRouter(parentRouteMsg, testRouterNotifier)
	assert.False(t, calledNotifier)

	parentR.Neighbor = map[string]string{"c3": ""}
	assert.NotContains(t, defaultClusterRouter.ParentNeighbor, "c3")
	assert.True(t, defaultClusterRouter.updateParentNeighbor(&parentR))
	assert.Contains(t, defaultClusterRouter.ParentNeighbor, "c3")
	assert.False(t, defaultClusterRouter.updateParentNeighbor(&parentR))
}

/*
TestRoute test route add and del.
route in test:
root
	- c1
		- cn
		- cm
	- c3
		- c2
*/
func TestRoute(t *testing.T) {
	r := Router()

	err := r.AddRoute("c1", "c1")
	assert.Nil(t, err)
	assert.Equal(t, "c1", defaultClusterRouter.subtreeRouter["c1"])
	assert.True(t, r.HasRoute("c1", "c1"))

	err = r.AddRoute("c2", "c1")
	assert.Nil(t, err)
	assert.Equal(t, "c1", defaultClusterRouter.subtreeRouter["c2"])

	err = r.AddRoute("c2", "c3")
	assert.NotNil(t, err)

	r.DelRoute("c2", "c1")
	assert.NotContains(t, defaultClusterRouter.subtreeRouter, "c2")
	assert.False(t, r.HasRoute("c2", "c1"))

	r.DelRoute("c1", "c1")
	assert.NotContains(t, defaultClusterRouter.subtreeRouter, "c1")

	err = r.AddRoute("c1", "c1")
	assert.Nil(t, err)

	err = r.AddRoute("c2", "c3")
	assert.Nil(t, err)

	err = r.AddRoute("c3", "c3")
	assert.Nil(t, err)

	err = r.AddRoute("cn", "c1")
	assert.Nil(t, err)

	err = r.AddRoute("cm", "c1")
	assert.Nil(t, err)

	assert.EqualValues(t,
		map[string][]string{
			"c1": {"cn"},
			"c3": {"c2"},
		},
		r.PortsToSubtreeClusters(&[]string{"c2", "cn"}),
	)
	assert.ElementsMatch(t,
		[]string{"c1", "c2", "c3", "cn", "cm"},
		r.SubTreeClusters())

	msg := r.SubTreeMessage()
	assert.NotNil(t, msg)
	assert.Equal(t, clustermessage.CommandType_SubTreeRoute, msg.Head.Command)
	serial, err := r.subtreeRouter.Serialize()
	assert.Nil(t, err)
	assert.Equal(t, serial, msg.Body)
	subr := SubtreeFromClusterController(msg)
	assert.Equal(t, r.subtreeRouter, subr)
}

func testRouterNotifier(msg *clustermessage.ClusterMessage, tos ...string) {
	calledNotifier = true
}
