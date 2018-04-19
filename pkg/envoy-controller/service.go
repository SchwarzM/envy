package envoy_controller

import (
	"strconv"
	"time"

	ev2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	elis "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"

	"github.com/schwarzm/envy/pkg/global"
)

func (e *EnvoyController) AddOrUpdateService(service *v1.Service, key string) error {
	glog.Infof("Update service: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	cluster := makeCluster(key)
	e.inventory.Clusters[key] = cluster
	ip := service.GetLabels()[global.ServiceLabel]
	listeners := make([]*ev2.Listener, 0)
	for _, port := range service.Spec.Ports {
		if port.Protocol == v1.ProtocolTCP {
			listeners = append(listeners, makeTCPListener(ip, port.Port, key))
		} else {
			glog.Infof("Ignoring service %v, port: %d due to UDP", key, port.Port)
		}
	}
	e.inventory.Listeners[key] = listeners
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func (e *EnvoyController) DeleteService(key string) error {
	glog.Infof("Delete service: %v", key)
	e.inventory.Mutex.Lock()
	defer e.inventory.Mutex.Unlock()
	e.inventory.SetVersion()
	delete(e.inventory.Listeners, key)
	delete(e.inventory.Clusters, key)
	return e.config.SetSnapshot(e.inventory.Node, e.inventory.Snapshot())
}

func makeCluster(key string) *ev2.Cluster {
	var cluster ev2.Cluster
	cluster.Name = key
	cluster.ConnectTimeout = time.Second * 5
	cluster.Type = ev2.Cluster_EDS
	cluster.EdsClusterConfig = &ev2.Cluster_EdsClusterConfig{
		EdsConfig: &core.ConfigSource{
			ConfigSourceSpecifier: &core.ConfigSource_Ads{
				Ads: &core.AggregatedConfigSource{},
			},
		},
	}
	return &cluster
}

func makeTCPListener(ip string, port int32, key string) *ev2.Listener {
	config := &tcp.TcpProxy{
		StatPrefix: key,
		Cluster:    key,
	}
	pbst, _ := util.MessageToStruct(config)
	listener := &ev2.Listener{
		Name: key + strconv.FormatInt(int64(port), 10),
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
	return listener
}
