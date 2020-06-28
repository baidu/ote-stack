package loadbalancer

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"sort"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

// setServers set the discovery server list in load balancer.
func (lb *LoadBalancer) setServers(serverAddresses []string) bool {
	serverAddresses, hasOriginalServer := sortServers(serverAddresses, lb.originalServerAddress)
	if len(serverAddresses) == 0 {
		return false
	}

	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if reflect.DeepEqual(serverAddresses, lb.ServerAddresses) {
		return false
	}

	lb.ServerAddresses = serverAddresses
	lb.randomServers = append([]string{}, lb.ServerAddresses...)
	rand.Shuffle(len(lb.randomServers), func(i, j int) {
		lb.randomServers[i], lb.randomServers[j] = lb.randomServers[j], lb.randomServers[i]
	})
	if !hasOriginalServer {
		lb.randomServers = append(lb.randomServers, lb.originalServerAddress)
	}
	lb.currentServerAddress = lb.randomServers[0]
	lb.nextServerIndex = 1

	return true
}

// nextServer get the next backup server in load balancer's list.
func (lb *LoadBalancer) nextServer(failedServer string) (string, error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if len(lb.randomServers) == 0 {
		return "", fmt.Errorf("No servers in load balancer proxy list")
	}
	if len(lb.randomServers) == 1 {
		return lb.currentServerAddress, nil
	}
	if failedServer != lb.currentServerAddress {
		return lb.currentServerAddress, nil
	}
	if lb.nextServerIndex >= len(lb.randomServers) {
		lb.nextServerIndex = 0
	}

	lb.currentServerAddress = lb.randomServers[lb.nextServerIndex]
	lb.nextServerIndex++

	return lb.currentServerAddress, nil
}

// sortServers check if the searched address in input list, and return
// the sorted list server address.
func sortServers(input []string, search string) ([]string, bool) {
	result := []string{}
	found := false
	skip := map[string]bool{"": true}

	for _, entry := range input {
		if skip[entry] {
			continue
		}
		if search == entry {
			found = true
		}
		skip[entry] = true
		result = append(result, entry)
	}

	sort.Strings(result)
	return result, found
}

// getAddresses return address list from endpoint resource.
func getAddresses(endpoint *v1.Endpoints) []string {
	serverAddresses := []string{}
	if endpoint == nil {
		return serverAddresses
	}
	for _, subset := range endpoint.Subsets {
		var port string
		if len(subset.Ports) > 0 {
			port = strconv.Itoa(int(subset.Ports[0].Port))
		}
		if port == "" {
			port = "443"
		}
		for _, address := range subset.Addresses {
			serverAddresses = append(serverAddresses, net.JoinHostPort(address.IP, port))
		}
	}
	return serverAddresses
}
