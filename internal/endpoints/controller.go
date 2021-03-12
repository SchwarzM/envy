package endpoints

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes"

	"github.com/envoyproxy/go-control-plane/pkg/wellknown"

	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"google.golang.org/protobuf/types/known/durationpb"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	tcp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	udp "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/udp/udp_proxy/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/go-logr/logr"
	"github.com/schwarzm/envy/internal/envy"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	Client client.Client
	Inv    cache.SnapshotCache
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContext(ctx)
	endpoint := core.Endpoints{}
	err := c.Client.Get(ctx, req.NamespacedName, &endpoint)
	if errors.IsNotFound(err) {
		log.Error(err, "could not find endpoint")
		return ctrl.Result{}, nil
	}
	if err != nil {
		log.Error(err, "could not fetch endpoint")
		return ctrl.Result{}, fmt.Errorf("could not fetch endpoint: %+v", err)
	}
	value, ok := endpoint.ObjectMeta.Labels[envy.ServiceLabel]
	if !ok {
		log.V(10).Info("endpoint has no envy label: ignoring")
		return ctrl.Result{}, nil
	}
	log.Info("label found", "label", value)
	// at this point we know that the endpoint that changed belonged to a service we care about
	svcList := core.ServiceList{}
	sel := labels.NewSelector()
	requirement, err := labels.NewRequirement(envy.ServiceLabel, selection.Exists, []string{})
	if err != nil {
		log.Error(err, "error creating requirement")
		return ctrl.Result{}, err
	}
	sel.Add(*requirement)
	listOptions := client.MatchingLabelsSelector{Selector: sel}
	err = c.Client.List(ctx, &svcList, listOptions)
	if err != nil {
		log.Error(err, "error listing services")
		return ctrl.Result{}, err
	}
	var clusters []types.Resource
	var listeners []types.Resource
	for _, svc := range svcList.Items {
		ip, ok := svc.ObjectMeta.Labels[envy.ServiceLabel]
		if !ok {
			log.Info("ignoring service")
			continue
		}
		cluster, err := makeCluster(svc)
		if err != nil {
			log.Error(err, "error making cluster")
			continue
		}
		clusters = append(clusters, cluster)
		listener, err := makeListener(svc, ip)
		if err != nil {
			log.Error(err, "error making listener")
			continue
		}
		for _, lis := range listener {
			listeners = append(listeners, lis)
		}
	}

	// endpoints aka cluster load assignment
	endpList := core.EndpointsList{}
	var clas []types.Resource
	err = c.Client.List(ctx, &endpList, listOptions)
	if err != nil {
		log.Error(err, "error listing endpoints")
		return ctrl.Result{}, err
	}
	for _, endp := range endpList.Items {
		_, ok := endp.ObjectMeta.Labels[envy.ServiceLabel]
		if !ok {
			log.Info("ignoring endp")
			continue
		}
		cla, err := makeEndpoint(endp)
		if err != nil {
			log.Error(err, "error making endpoints")
			return ctrl.Result{}, err
		}
		clas = append(clas, cla)
	}
	snap, err := c.Inv.GetSnapshot("lb")
	if err != nil {
		log.Error(err, "error getting snapshot")
		return ctrl.Result{}, err
	}
	version := snap.GetVersion(envy.TypeURL)
	intv, err := strconv.Atoi(version)
	if err != nil {
		log.Error(err, "error converting version to int")
		return ctrl.Result{}, err
	}
	intv++
	snap = cache.NewSnapshot(strconv.Itoa(intv), clas, clusters, nil, listeners, nil, nil)
	err = c.Inv.SetSnapshot("lb", snap)
	if err != nil {
		log.Error(err, "error setting snapshot")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func makeListener(svc core.Service, ip string) ([]types.Resource, error) {
	if ip == "" {
		return nil, fmt.Errorf("error creating listener")
	}
	configTcp := &tcp.TcpProxy{
		StatPrefix: "tcp",
		ClusterSpecifier: &tcp.TcpProxy_Cluster{
			Cluster: makeName(svc.Namespace, svc.Name),
		},
	}
	configUdp := &udp.UdpProxyConfig{
		RouteSpecifier: &udp.UdpProxyConfig_Cluster{
			Cluster: makeName(svc.Namespace, svc.Name),
		},
	}
	tcpConfig, err := ptypes.MarshalAny(configTcp)
	if err != nil {
		return nil, err
	}
	udpConfig, err := ptypes.MarshalAny(configUdp)
	if err != nil {
		return nil, err
	}
	var ret []types.Resource
	for _, port := range svc.Spec.Ports {
		if port.Protocol == core.ProtocolTCP {
			listener := listenerservice.Listener{
				Name: makeServiceName(svc.Namespace, svc.Name, port.Port),
				Address: &envoycore.Address{
					Address: &envoycore.Address_SocketAddress{
						SocketAddress: &envoycore.SocketAddress{
							Address:  ip,
							Protocol: envoycore.SocketAddress_TCP,
							PortSpecifier: &envoycore.SocketAddress_PortValue{
								PortValue: uint32(port.Port),
							},
						},
					},
				},
				Freebind: &wrappers.BoolValue{
					Value: true,
				},
				FilterChains: []*listenerservice.FilterChain{
					{
						Filters: []*listenerservice.Filter{
							{
								Name: wellknown.TCPProxy,
								ConfigType: &listenerservice.Filter_TypedConfig{
									TypedConfig: tcpConfig,
								},
							},
						},
					},
				},
			}
			ret = append(ret, &listener)
		} else if port.Protocol == core.ProtocolUDP {
			listener := listenerservice.Listener{
				Name: makeServiceName(svc.Namespace, svc.Name, port.Port),
				Address: &envoycore.Address{
					Address: &envoycore.Address_SocketAddress{
						SocketAddress: &envoycore.SocketAddress{
							Address:  ip,
							Protocol: envoycore.SocketAddress_UDP,
							PortSpecifier: &envoycore.SocketAddress_PortValue{
								PortValue: uint32(port.Port),
							},
						},
					},
				},
				Freebind: &wrappers.BoolValue{
					Value: true,
				},
				ListenerFilters: []*listenerservice.ListenerFilter{
					{
						Name: "envoy.filters.udp_listener.udp_proxy",
						ConfigType: &listenerservice.ListenerFilter_TypedConfig{
							TypedConfig: udpConfig,
						},
					},
				},
			}
			ret = append(ret, &listener)
		}
	}
	return ret, nil
}

func makeCluster(svc core.Service) (types.Resource, error) {
	grpcService := &envoycore.GrpcService_EnvoyGrpc_{
		EnvoyGrpc: &envoycore.GrpcService_EnvoyGrpc{
			ClusterName: "xds_cluster",
		},
	}
	ret := cluster.Cluster{
		Name: makeName(svc.Namespace, svc.Name),
		ConnectTimeout: &durationpb.Duration{
			Seconds: 10,
		},
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_EDS},
		EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{
			EdsConfig: &envoycore.ConfigSource{
				ResourceApiVersion: envoycore.ApiVersion_V3,
				ConfigSourceSpecifier: &envoycore.ConfigSource_ApiConfigSource{
					ApiConfigSource: &envoycore.ApiConfigSource{
						ApiType:             envoycore.ApiConfigSource_GRPC,
						TransportApiVersion: envoycore.ApiVersion_V3,
						GrpcServices: []*envoycore.GrpcService{
							{
								TargetSpecifier: grpcService,
							},
						},
						SetNodeOnFirstMessageOnly: true,
					},
				},
			},
		},
	}
	return &ret, nil
}

func makeEndpoint(endp core.Endpoints) (types.Resource, error) {
	lbendp, err := makeLbEndpoints(endp)
	if err != nil {
		return nil, err
	}
	ret := endpoint.ClusterLoadAssignment{
		ClusterName: makeName(endp.Namespace, endp.Name),
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: lbendp,
		}},
	}
	return &ret, nil
}

func makeLbEndpoints(endp core.Endpoints) ([]*endpoint.LbEndpoint, error) {
	var ret []*endpoint.LbEndpoint

	for _, subset := range endp.Subsets {
		for _, port := range subset.Ports {
			for _, addr := range subset.Addresses {
				rep, err := makeRealEndpont(addr.IP, port.Protocol, port.Port)
				if err != nil {
					return nil, err
				}
				ret = append(ret, &rep)
			}
		}
	}

	return ret, nil
}

func makeRealEndpont(ip string, proto core.Protocol, port int32) (endpoint.LbEndpoint, error) {
	ret := endpoint.LbEndpoint{}
	var p envoycore.SocketAddress_Protocol
	if proto == core.ProtocolTCP {
		p = envoycore.SocketAddress_TCP
	} else if proto == core.ProtocolUDP {
		p = envoycore.SocketAddress_UDP
	} else {
		return ret, fmt.Errorf("protocol not implemented %s", proto)
	}
	ret.HostIdentifier = &endpoint.LbEndpoint_Endpoint{
		Endpoint: &endpoint.Endpoint{
			Address: &envoycore.Address{
				Address: &envoycore.Address_SocketAddress{
					SocketAddress: &envoycore.SocketAddress{
						Protocol: p,
						Address:  ip,
						PortSpecifier: &envoycore.SocketAddress_PortValue{
							PortValue: uint32(port),
						},
					},
				},
			},
		},
	}
	return ret, nil
}

func makeName(namespace string, name string) string {
	return strings.Join([]string{namespace, name}, "/")
}

func makeServiceName(namespace string, name string, port int32) string {
	pstr := strconv.Itoa(int(port))
	return strings.Join([]string{namespace, name, pstr}, "/")
}
