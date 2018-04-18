package envoy_controller

import (
	ev2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	elis "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
)

func (e *EnvoyController) AddOrUpdateService(service *v1.Service, key string) error {
	glog.Infof("Update service: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	cluster := makeCluster(key)
	e.inventory.Clusters[key] = cluster
	e.inventory.Listeners[key], _ = makeTCPListener(service, key)
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func (e *EnvoyController) DeleteService(key string) error {
	glog.Infof("Delete service: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	delete(e.inventory.Listeners, key)
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func makeTCPListener(service *v1.Service, key string) (*ev2.Listener, error) {
	//TODO: use real servicelabel here, cyclic dep ?
	ip := service.GetLabels()["schwarzm/envy"]
	//TODO: more than 1 port?
	port := service.Spec.Ports[0].Port
	config := &tcp.TcpProxy{
		StatPrefix: key,
		Cluster:    key,
	}
	pbst, err := util.MessageToStruct(config)
	if err != nil {
		return nil, err
	}
	listener := &ev2.Listener{
		Name: key,
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  ip,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: uint32(port),
					},
				},
			},
		},
		FilterChains: []elis.FilterChain{
			{
				Filters: []elis.Filter{
					{
						Name:   util.TCPProxy,
						Config: pbst,
					},
				},
			},
		},
	}
	return listener, nil
}
