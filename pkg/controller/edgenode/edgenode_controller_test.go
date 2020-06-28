package edgenode

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	otev1 "github.com/baidu/ote-stack/pkg/apis/ote/v1"
	otefake "github.com/baidu/ote-stack/pkg/generated/clientset/versioned/fake"
)

var (
	nodeGroup     = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}
	edgenodeGroup = schema.GroupVersionResource{Group: "ote.baidu.com", Version: "v1", Resource: "edgenodes"}
)

func newReadyNode() *corev1.Node {
	var ret = []corev1.NodeCondition{}
	condition := corev1.NodeCondition{}

	condition.Type = corev1.NodeReady
	condition.Status = corev1.ConditionTrue
	condition.Reason = "Ready"
	condition.Message = "Ready"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: corev1.NodeStatus{
			Conditions: ret,
		},
	}
}

func newNotReadNode() *corev1.Node {
	var ret = []corev1.NodeCondition{}
	condition := corev1.NodeCondition{}

	condition.Type = corev1.NodeReady
	condition.Status = corev1.ConditionUnknown
	condition.Reason = "NotReady"
	condition.Message = "NotReady"
	condition.LastHeartbeatTime = metav1.Now()
	ret = append(ret, condition)

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Status: corev1.NodeStatus{
			Conditions: ret,
		},
	}
}

func newEdgeNode() *otev1.EdgeNode {
	return &otev1.EdgeNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Status: "NotReady",
	}
}

func newNodePatchAction(name string, data []byte) kubetesting.PatchActionImpl {
	return kubetesting.NewPatchSubresourceAction(nodeGroup, "", name, types.StrategicMergePatchType, data, "status")
}

func newEdgeNodeGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(edgenodeGroup, "kube-system", name)
}

func newEdgeNodeCreateAction(en *otev1.EdgeNode) kubetesting.CreateActionImpl {
	return kubetesting.NewCreateAction(edgenodeGroup, "kube-system", en)
}

func newEdgeNodeUpdateAction(en *otev1.EdgeNode) kubetesting.UpdateActionImpl {
	return kubetesting.NewUpdateAction(edgenodeGroup, "kube-system", en)
}

func newEdgeNodeDeleteAction(name string) kubetesting.DeleteActionImpl {
	return kubetesting.NewDeleteAction(edgenodeGroup, "kube-system", name)
}

func TestHandleNodeAdd(t *testing.T) {
	ctx := &ControllerContext{}
	node1 := newReadyNode()
	node2 := newNotReadNode()
	edgeNode := newEdgeNode()

	testCase := []struct {
		name              string
		handleNode        *corev1.Node
		getEdgeNodeResult *otev1.EdgeNode
		errorOnGet        error
		errorOnCreate     error
		expectActions     []kubetesting.Action
	}{
		{
			name:              "edgenode is not exist in server.",
			handleNode:        node1,
			getEdgeNodeResult: nil,
			errorOnGet:        kubeerrors.NewNotFound(schema.GroupResource{}, ""),
			errorOnCreate:     nil,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(edgeNode.Name), newEdgeNodeCreateAction(edgeNode),
			},
		},
		{
			name:              "edgenode is exist in server.",
			handleNode:        node1,
			getEdgeNodeResult: edgeNode,
			errorOnGet:        nil,
			errorOnCreate:     nil,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(edgeNode.Name),
			},
		},
		{
			name:              "a not ready node is exist in server.",
			handleNode:        node2,
			getEdgeNodeResult: edgeNode,
			errorOnGet:        nil,
			errorOnCreate:     nil,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(edgeNode.Name),
				newEdgeNodeGetAction(edgeNode.Name),
			},
		},
	}

	for _, test := range testCase {
		mockOteClient := &otefake.Clientset{}
		mockOteClient.AddReactor("get", "edgenodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.getEdgeNodeResult, test.errorOnGet
		})
		mockOteClient.AddReactor("create", "edgenodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, nil, test.errorOnCreate
		})

		ctx.OteClient = mockOteClient
		controller := NewEdgeNodeController(ctx)
		controller.handleNodeAdd(test.handleNode)
		assert.Equal(t, test.expectActions, mockOteClient.Actions())
	}
}

func TestHandleNodeUpdate(t *testing.T) {
	ctx := &ControllerContext{}
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	node2 := newNotReadNode()
	node3 := newReadyNode()
	egdeNode1 := newEdgeNode()

	testCase := []struct {
		name              string
		handleNode        *corev1.Node
		getEdgeNodeResult *otev1.EdgeNode
		errorOnGet        error
		errorOnUpdate     error
		expectActions     []kubetesting.Action
	}{
		{
			name:              "node condition is null",
			handleNode:        node1,
			getEdgeNodeResult: egdeNode1,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(egdeNode1.Name),
			},
		},
		{
			name:              "node condition is unknown",
			handleNode:        node2,
			getEdgeNodeResult: egdeNode1,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(egdeNode1.Name),
			},
		},
		{
			name:              "node is ready",
			handleNode:        node3,
			getEdgeNodeResult: egdeNode1,
			expectActions: []kubetesting.Action{
				newEdgeNodeGetAction(egdeNode1.Name),
				newEdgeNodeUpdateAction(egdeNode1),
			},
		},
	}

	for _, test := range testCase {
		mockOteClient := &otefake.Clientset{}
		mockOteClient.AddReactor("get", "edgenodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, test.getEdgeNodeResult, test.errorOnGet
		})
		mockOteClient.AddReactor("update", "edgenodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, nil, test.errorOnUpdate
		})

		ctx.OteClient = mockOteClient
		controller := NewEdgeNodeController(ctx)
		controller.handleNodeUpdate(test.handleNode)
		assert.Equal(t, test.expectActions, mockOteClient.Actions())
	}
}

func TestHandleDeleteNode(t *testing.T) {
	ctx := &ControllerContext{}
	node := newReadyNode()

	testCase := []struct {
		name          string
		handleNode    *corev1.Node
		errorOnDelete error
		expectActions []kubetesting.Action
	}{
		{
			name:       "delete node",
			handleNode: node,
			expectActions: []kubetesting.Action{
				newEdgeNodeDeleteAction(node.Name),
			},
		},
	}

	for _, test := range testCase {
		mockOteClient := &otefake.Clientset{}
		mockOteClient.AddReactor("delete", "edgenodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
			return true, nil, test.errorOnDelete
		})

		ctx.OteClient = mockOteClient
		controller := NewEdgeNodeController(ctx)
		controller.handleNodeDelete(test.handleNode)
		assert.Equal(t, test.expectActions, mockOteClient.Actions())
	}
}

func TestUpdateNodeStatus(t *testing.T) {
	ctx := &ControllerContext{}
	data := getNewNodeConditionData()

	mockKubeClient := &k8sfake.Clientset{}
	mockKubeClient.AddReactor("patch", "nodes", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})
	ctx.KubeClient = mockKubeClient

	controller := NewEdgeNodeController(ctx)
	controller.addUpdatedNode("test")
	controller.updateNodeStatus()

	expectActions := []kubetesting.Action{
		newNodePatchAction("test", data),
	}
	assert.Equal(t, expectActions, mockKubeClient.Actions())
}
