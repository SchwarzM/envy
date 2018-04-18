package envoy_controller

import (
	"sync"
	"time"

	ev2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/golang/glog"
)

const NodeName = "node"

type Inventory struct {
	Mutex     sync.Mutex
	Version   string
	Listeners map[string]*ev2.Listener
	Clas      map[string]*ev2.ClusterLoadAssignment
	Clusters  map[string]*ev2.Cluster
	Node      string
}

func NewInventory() *Inventory {
	return &Inventory{
		Version:   "",
		Listeners: make(map[string]*ev2.Listener),
		Clas:      make(map[string]*ev2.ClusterLoadAssignment),
		Clusters:  make(map[string]*ev2.Cluster),
		Node:      NodeName,
	}
}

func (i *Inventory) SetVersion() {
	glog.V(10).Infof("We are currently at version: %v", i.Version)
	i.Version = time.Now().String()
	glog.V(10).Infof("Setting new version at: %v", i.Version)
}

func (i *Inventory) Snapshot() cache.Snapshot {
	listeners := make([]cache.Resource, len(i.Listeners))
	count := 0
	for _, listener := range i.Listeners {
		listeners[count] = listener
		count = count + 1
	}
	clusters := make([]cache.Resource, len(i.Clusters))
	count = 0
	for _, cluster := range i.Clusters {
		clusters[count] = cluster
		count = count + 1
	}
	endpoints := make([]cache.Resource, len(i.Clas))
	count = 0
	for _, endp := range i.Clas {
		endpoints[count] = endp
		count = count + 1
	}
	return cache.NewSnapshot(i.Version, endpoints, clusters, nil, listeners)
}
