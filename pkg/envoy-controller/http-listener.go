package envoy_controller

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	//als "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	//alf "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/envoyproxy/go-control-plane/pkg/util"
)

func MakeHTTPListener(listenerName string, addr string, route string, port uint32) *v2.Listener {

	//Route configuration
	rdsSource := core.ConfigSource{}
	rdsSource.ConfigSourceSpecifier = &core.ConfigSource_Ads{
		Ads: &core.AggregatedConfigSource{},
	}

	// access log service configuration
	//alsConfig := &als.HttpGrpcAccessLogConfig{
	//	CommonConfig: &als.CommonGrpcAccessLogConfig{
	//		LogName: "echo",
	//		GrpcService: &core.GrpcService{
	//			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
	//				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
	//					ClusterName: "xds_cluster",
	//				},
	//			},
	//		},
	//	},
	//}
	//alsConfigPbst, err := util.MessageToStruct(alsConfig)
	//if err != nil {
	//	panic(err)
	//}

	// HTTP filter configuration
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    rdsSource,
				RouteConfigName: route,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: util.Router,
		}},
		//AccessLog: []*alf.AccessLog{{
		//	Name:   util.HTTPGRPCAccessLog,
		//	Config: alsConfigPbst,
		//}},
	}
	pbst, err := util.MessageToStruct(manager)
	if err != nil {
		panic(err)
	}

	return &v2.Listener{
		Name: listenerName,
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  addr,
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: port,
					},
				},
			},
		},
		FilterChains: []listener.FilterChain{{
			Filters: []listener.Filter{{
				Name:   util.HTTPConnectionManager,
				Config: pbst,
			}},
		}},
	}
}
