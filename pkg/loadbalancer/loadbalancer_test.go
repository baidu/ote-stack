package loadbalancer

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
)

var (
	endpointGroup = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "endpoints"}
)

func newEndpointGetAction(name string) kubetesting.GetActionImpl {
	return kubetesting.NewGetAction(endpointGroup, "default", name)
}

func TestFindBackend(t *testing.T) {
	ep := newEndpoint()
	ep.Name = "kubernetes"
	mockClient := &k8sfake.Clientset{}
	mockClient.AddReactor("get", "endpoints", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, ep, nil
	})

	lb := &LoadBalancer{
		config: &Config{
			KubeClient: mockClient,
		},
	}

	lb.findBackend()

	expecteActions := []kubetesting.Action{
		newEndpointGetAction(ep.Name),
	}
	assert.Equal(t, expecteActions, mockClient.Actions())
	assert.Equal(t, []string{"1.1.1.1:80"}, lb.ServerAddresses)
}

func TestSendSignalToSyncer(t *testing.T) {
	channel := make(chan bool)
	lb := &LoadBalancer{
		config: &Config{
			HealthChan: channel,
		},
	}

	// set remote ready
	go func() {
		status := <-channel
		assert.Equal(t, true, status)
	}()

	lb.sendSignalToSyncer(RemoteReady)
	assert.Equal(t, true, lb.IsRemoteEnable())

	// set remote not ready
	go func() {
		status := <-channel
		assert.Equal(t, false, status)
	}()

	lb.sendSignalToSyncer(RemoteNotReady)
	assert.Equal(t, false, lb.IsRemoteEnable())
}

func TestHealthCheck(t *testing.T) {
	channel := make(chan bool)
	lb := &LoadBalancer{
		randomServers:      []string{"127.0.0.1:5999"},
		serverHealthStatus: make(map[string]bool),
		config: &Config{
			HealthChan: channel,
		},
	}

	// health check success
	go func() {
		status := <-channel
		assert.Equal(t, true, status)
	}()

	lb.healthCheck()
	assert.NotNil(t, lb.serverHealthStatus["127.0.0.1:5999"])
	assert.Equal(t, true, lb.serverHealthStatus["127.0.0.1:5999"])

	// health check fail
	lb.randomServers = []string{"127.0.0.1:4999"}
	go func() {
		status := <-channel
		assert.Equal(t, false, status)
	}()

	lb.healthCheck()
	assert.NotNil(t, lb.serverHealthStatus["127.0.0.1:4999"])
	assert.Equal(t, false, lb.serverHealthStatus["127.0.0.1:4999"])
}

func TestGetAvailablebackend(t *testing.T) {
	lb := &LoadBalancer{
		serverHealthStatus: make(map[string]bool),
	}
	lb.serverHealthStatus["127.0.0.1:5999"] = true
	lb.setServers([]string{"127.0.0.1:5999"})

	// success get backend
	addr, conn, err := lb.getAvailableBackend()
	assert.Nil(t, err)
	assert.Equal(t, "127.0.0.1:5999", addr)
	assert.NotNil(t, conn)

	// fail to get backend
	lb.setServers([]string{"127.0.0.1:4999"})
	lb.serverHealthStatus["127.0.0.1:4999"] = false

	addr, conn, err = lb.getAvailableBackend()
	assert.NotNil(t, err)
	assert.Nil(t, conn)
	assert.Equal(t, "", addr)

	// fail to get all backend
	lb.setServers([]string{"127.0.0.1:3999", "127.0.0.1:4999"})
	lb.serverHealthStatus["127.0.0.1:4999"] = true
	lb.serverHealthStatus["127.0.0.1:3999"] = true

	addr, conn, err = lb.getAvailableBackend()
	assert.NotNil(t, err)
	assert.Nil(t, conn)
	assert.Equal(t, "", addr)
}

func TestTCPProxy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:6999")
	assert.Nil(t, err)
	assert.NotNil(t, listener)

	go func() {
		i := 0
		for {
			i++
			conn, err := listener.Accept()
			if err != nil {
				break
			}

			buffer := make([]byte, 5)
			conn.Read(buffer)
			// skip the first read content, because it is the header.
			if i == 2 {
				assert.Equal(t, []byte("hello"), buffer)
			}

			conn.Close()
		}
	}()

	// wait for 127.0.0.1:6999 health check done
	time.Sleep(time.Second * 3)
	c, err := net.DialTimeout("tcp", "127.0.0.1:5999", DialTimeOut)
	assert.Nil(t, err)
	assert.NotNil(t, c)

	c.Write([]byte("hello"))
	time.Sleep(time.Second * 3)

	c.Close()
	listener.Close()
}

func TestMain(m *testing.M) {
	stopChan := make(chan struct{})
	healthChan := make(chan bool)

	config := &Config{
		LbAddress:     "127.0.0.1:5999",
		StopChan:      stopChan,
		HealthChan:    healthChan,
		ServerAddress: "127.0.0.1:6999",
		KubeClient:    &k8sfake.Clientset{},
	}

	_, err := Start(config)
	if err != nil {
		return
	}

	go func() {
		for {
			select {
			case <-healthChan:
			}
		}
	}()

	exit := m.Run()
	close(stopChan)
	close(healthChan)
	os.Exit(exit)
}
