package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestFormKeyName(t *testing.T) {
	key := FormKeyName("ns1", "name1")
	assert.Equal(t, "ns1/name1", key)

	key = FormKeyName("", "name1")
	assert.Equal(t, "name1", key)
}

func TestGetObjectFromSerializeData(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	data, err := json.Marshal(pod)
	assert.Nil(t, err)
	assert.NotNil(t, data)

	obj, err := GetObjectFromSerializeData(ResourcePod, data)
	assert.Nil(t, err)
	assert.Equal(t, "test", obj.(*corev1.Pod).Name)

	obj, err = GetObjectFromSerializeData("ingress", data)
	assert.NotNil(t, err)
	assert.Nil(t, obj)
}
