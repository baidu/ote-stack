package loadbalancer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newEndpoint() *corev1.Endpoints {
	addr := corev1.EndpointAddress{
		IP: "1.1.1.1",
	}
	port := corev1.EndpointPort{
		Port: 80,
	}

	s := corev1.EndpointSubset{
		Addresses: []corev1.EndpointAddress{
			addr,
		},
		Ports: []corev1.EndpointPort{
			port,
		},
	}

	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{s},
	}
}

func TestSetServers(t *testing.T) {
	lb := LoadBalancer{
		ServerAddresses:       []string{"1.1.1.1"},
		originalServerAddress: "1.1.1.1",
	}

	serverList := []string{"1.1.1.1"}
	result := lb.setServers(serverList)
	assert.Equal(t, false, result)
	assert.Equal(t, []string{"1.1.1.1"}, lb.ServerAddresses)
	assert.Equal(t, 0, len(lb.randomServers))

	serverList = []string{"2.2.2.2"}
	result = lb.setServers(serverList)
	assert.Equal(t, true, result)
	assert.Equal(t, []string{"2.2.2.2"}, lb.ServerAddresses)
	assert.Equal(t, 2, len(lb.randomServers))
}

func TestNextServer(t *testing.T) {
	lb := LoadBalancer{
		randomServers:        []string{"1.1.1.1", "2.2.2.2"},
		currentServerAddress: "1.1.1.1",
		nextServerIndex:      1,
	}

	addr, err := lb.nextServer("1.1.1.1")
	assert.Nil(t, err)
	assert.Equal(t, "2.2.2.2", addr)
	assert.Equal(t, 2, lb.nextServerIndex)
}

func TestSortServer(t *testing.T) {
	list := []string{"2.2.2.2", "1.1.1.1", "3.3.3.3"}

	result, found := sortServers(list, "1.1.1.1")
	assert.Equal(t, true, found)
	assert.Equal(t, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, result)

	result, found = sortServers(list, "4.4.4.4")
	assert.Equal(t, false, found)
	assert.Equal(t, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, result)
}

func TestGetAddress(t *testing.T) {
	ep := newEndpoint()
	list := getAddresses(ep)
	assert.Equal(t, []string{"1.1.1.1:80"}, list)
}
