// Package loadbalancer provides methods of finding available backends.
package loadbalancer

import (
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtypes "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

const (
	RemoteReady    = "ready"
	RemoteNotReady = "not_ready"

	ReadBuffer        = 4096000
	DialTimeOut       = time.Second * 10
	HealthCheckPeriod = time.Second * 2
)

type LbConnection map[net.Conn]struct{}

type LoadBalancer struct {
	mutex              sync.RWMutex
	tcpConnection      map[string]LbConnection
	serverHealthStatus map[string]bool

	originalServerAddress string
	nextServerIndex       int
	currentServerAddress  string
	ServerAddresses       []string
	randomServers         []string

	remoteEnable bool
	config       *Config
}

type Config struct {
	LbAddress     string
	ServerAddress string
	KubeClient    kubernetes.Interface
	HealthChan    chan bool
	StopChan      chan struct{}
}

// Start finds available backend servers and return a loadbalancer.
func Start(config *Config) (_lb *LoadBalancer, _err error) {
	lb := &LoadBalancer{
		config:                config,
		originalServerAddress: config.ServerAddress,
		remoteEnable:          false,
		tcpConnection:         make(map[string]LbConnection),
		serverHealthStatus:    make(map[string]bool),
	}

	lb.setServers([]string{lb.originalServerAddress})

	go lb.proxyStart()

	lb.findBackend()

	return lb, nil
}

// update sets the backend server list in load balancer.
func (lb *LoadBalancer) update(serverAddress []string) {
	if lb == nil {
		return
	}

	if !lb.setServers(serverAddress) {
		return
	}

	klog.V(2).Infof("Updating load balancer server address -> %v", lb.randomServers)
}

// IsRemoteEnable returns whether backend server is available.
func (lb *LoadBalancer) IsRemoteEnable() bool {
	return lb.remoteEnable
}

// findBackend searchs the available backend servers.
func (lb *LoadBalancer) findBackend() error {
	addresses := []string{}

	endpoint, _ := lb.config.KubeClient.CoreV1().Endpoints("default").Get("kubernetes", metav1.GetOptions{})
	if endpoint != nil {
		addresses = getAddresses(endpoint)
		lb.update(addresses)
	}

	go func() {
	connect:
		for {
			time.Sleep(5 * time.Second)
			watch, err := lb.config.KubeClient.CoreV1().Endpoints("default").Watch(metav1.ListOptions{
				FieldSelector:   fields.Set{"metadata.name": "kubernetes"}.String(),
				ResourceVersion: "0",
			})
			if err != nil {
				klog.Errorf("Unable to watch for loadbalancer endpoints: %v", err)
				continue connect
			}
		watching:
			for {
				select {
				case ev, ok := <-watch.ResultChan():
					if !ok || ev.Type == watchtypes.Error {
						if ok {
							klog.Errorf("loadbalancer endpoint watch channel closed: %v", ev)
						}
						watch.Stop()
						continue connect
					}
					endpoint, ok := ev.Object.(*v1.Endpoints)
					if !ok {
						klog.Errorf("loadbalancer could not event object to endpoint: %v", ev)
						continue watching
					}

					newAddresses := getAddresses(endpoint)
					if reflect.DeepEqual(newAddresses, addresses) {
						continue watching
					}
					addresses = newAddresses
					klog.Infof("loadbalancer endpoint watch event: %v", addresses)
					lb.update(addresses)
				}
			}
		}
	}()

	return nil
}

func (lb *LoadBalancer) sendSignalToSyncer(signal string) {
	switch signal {
	case RemoteReady:
		if !lb.remoteEnable {
			lb.config.HealthChan <- true
			lb.remoteEnable = true
		}
	case RemoteNotReady:
		if lb.remoteEnable {
			lb.remoteEnable = false
			lb.config.HealthChan <- false
		}
	default:
		klog.Errorf("unsupported signal type")
	}
}

func (lb *LoadBalancer) proxyStart() {
	lbListener, err := net.Listen("tcp", lb.config.LbAddress)
	if err != nil {
		klog.Errorf("Unable to listen on: %s, error: %v", lb.config.LbAddress, err)
		return
	}
	defer lbListener.Close()

	go wait.Until(lb.healthCheck, HealthCheckPeriod, lb.config.StopChan)

	for {
		proxyConn, err := lbListener.Accept()
		if err != nil {
			klog.Errorf("Unable to accept a request: %v", err)
			continue
		}

		targetAddr, targetConn, err := lb.getAvailableBackend()
		if err != nil {
			klog.Errorf("Unable to connect to: %s, error: %v", lb.currentServerAddress, err)
			proxyConn.Close()
			continue
		}

		lb.addTcpConnection(targetAddr, proxyConn, targetConn)

		go lb.proxyRequest(targetAddr, proxyConn, targetConn)
		go lb.proxyRequest(targetAddr, targetConn, proxyConn)
	}
}

// proxyRequest forwards all requests from r to w
func (lb *LoadBalancer) proxyRequest(ip string, r net.Conn, w net.Conn) {
	defer func() {
		r.Close()
		w.Close()
		lb.deleteTcpConnection(ip, r, w)
	}()

	var buffer = make([]byte, ReadBuffer)
	for {
		n, err := r.Read(buffer)
		if err != nil {
			klog.Errorf("Unable to read from input: %v", err)
			break
		}

		n, err = w.Write(buffer[:n])
		if err != nil {
			klog.Errorf("Unable to write to output: %v", err)
			break
		}
	}
}

func (lb *LoadBalancer) healthCheck() {
	isRemoteHealth := false

	for _, addr := range lb.getCurrentServerList() {
		if _, err := net.DialTimeout("tcp", addr, DialTimeOut); err != nil {
			for c := range lb.getTcpConnection(addr) {
				lb.deleteTcpConnection(addr, c)
			}
			lb.setHealthStatus(addr, false)
			klog.V(2).Infof("%s is not health", addr)
		} else {
			lb.setHealthStatus(addr, true)
			isRemoteHealth = true
			klog.V(2).Infof("%s is health", addr)
		}
	}

	if !isRemoteHealth {
		lb.sendSignalToSyncer(RemoteNotReady)
	} else {
		lb.sendSignalToSyncer(RemoteReady)
	}
}

func (lb *LoadBalancer) getAvailableBackend() (string, net.Conn, error) {
	startIndex := 0
	for {
		if startIndex == len(lb.randomServers) {
			return "", nil, fmt.Errorf("all servers failed")
		}
		startIndex++

		targetServer := lb.currentServerAddress
		// if current server is not health, then changes to the next server.
		if !lb.getHealthStatus(targetServer) {
			if _, err := lb.nextServer(targetServer); err != nil {
				return "", nil, err
			}
			continue
		}

		conn, err := net.DialTimeout("tcp", targetServer, DialTimeOut)
		if err == nil {
			klog.V(2).Infof("current request connect to: %s", targetServer)
			// the next request will uses the next server in randomServers
			lb.nextServer(targetServer)

			return targetServer, conn, nil
		}

		klog.Errorf("Dial error from load balancer: %v", err)

		// if current server couldn't connect, then uses the next server.
		if _, err = lb.nextServer(targetServer); err != nil {
			return "", nil, err
		}
	}
}

func (lb *LoadBalancer) addTcpConnection(addr string, conn ...net.Conn) {
	defer lb.mutex.Unlock()

	lb.mutex.Lock()
	if _, ok := lb.tcpConnection[addr]; !ok {
		c := make(LbConnection)
		lb.tcpConnection[addr] = c
	}

	for _, c := range conn {
		lb.tcpConnection[addr][c] = struct{}{}
	}
}

func (lb *LoadBalancer) deleteTcpConnection(addr string, conn ...net.Conn) {
	defer lb.mutex.Unlock()

	lb.mutex.Lock()
	for _, c := range conn {
		if c != nil {
			c.Close()
		}
		delete(lb.tcpConnection[addr], c)
	}
}

func (lb *LoadBalancer) getTcpConnection(addr string) LbConnection {
	defer lb.mutex.RUnlock()

	lb.mutex.RLock()
	if conn, ok := lb.tcpConnection[addr]; ok {
		return conn
	}
	return LbConnection{}
}

func (lb *LoadBalancer) setHealthStatus(addr string, status bool) {
	defer lb.mutex.Unlock()

	lb.mutex.Lock()
	lb.serverHealthStatus[addr] = status
}

func (lb *LoadBalancer) getHealthStatus(addr string) bool {
	defer lb.mutex.RUnlock()

	lb.mutex.RLock()
	if status, ok := lb.serverHealthStatus[addr]; ok {
		return status
	}
	return false
}

func (lb *LoadBalancer) getCurrentServerList() []string {
	defer lb.mutex.RUnlock()

	lb.mutex.RLock()
	return lb.randomServers
}
