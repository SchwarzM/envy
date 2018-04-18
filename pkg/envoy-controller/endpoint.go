package envoy_controller

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

func (e *EnvoyController) AddOrUpdateEndpoints(endpoints *v1.Endpoints, key string) error {
	glog.Infof("Update endpoint: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	cla := makeClusterLoadAssignment(key)
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			for _, port := range subset.Ports {
				host := makeEndpoint(addr.IP, port.Port, key)
				cla.Endpoints[0].LbEndpoints = append(cla.Endpoints[0].LbEndpoints, host)
			}
		}
	}
	e.inventory.Clas[key] = cla
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func (e *EnvoyController) DeleteEndpoints(key string) error {
	glog.Infof("Delete service: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	delete(e.inventory.Clusters, key)
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func makeClusterLoadAssignment(key string) *v2.ClusterLoadAssignment {
	var cla v2.ClusterLoadAssignment
	cla.ClusterName = key
	// Currently we just have one locality
	cla.Endpoints = make([]endpoint.LocalityLbEndpoints, 1)
	cla.Endpoints[0].LbEndpoints = make([]endpoint.LbEndpoint, 0)
	return &cla
}

func makeEndpoint(addr string, port int32, key string) endpoint.LbEndpoint {
	return endpoint.LbEndpoint{
		Endpoint: &endpoint.Endpoint{
			Address: &core.Address{
				Address: &core.Address_SocketAddress{
					SocketAddress: &core.SocketAddress{
						Protocol: core.TCP,
						Address:  addr,
						PortSpecifier: &core.SocketAddress_PortValue{
							PortValue: uint32(port),
						},
					},
				},
			},
		},
	}
}
