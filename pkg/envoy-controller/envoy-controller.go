package envoy_controller

import (
	//	"context"
	"net"
	"sync"
	//"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

type Logger struct{}

func (l Logger) Infof(format string, args ...interface{}) {
	//TODO: fixme
	glog.V(10).Infof("Logger: format: %v, args: %v", format, args)
	glog.Infof(format, args)
}

func (l Logger) Errorf(format string, args ...interface{}) {
	//TODO: fixme
	glog.Errorf(format, args)
}

// Hasher returns node ID as an ID
type Hasher struct{}

// ID function
func (h Hasher) ID(node *core.Node) string {
	if node == nil {
		return "unknown"
	}
	return node.Id
}

type EnvoyController struct {
	cb         *callbacks
	config     cache.SnapshotCache
	wg         *sync.WaitGroup
	addrString string
	stopCh     chan struct{}
	inventory  *Inventory
}

func NewEnvoyController(wg *sync.WaitGroup, addrString string, stopCh chan struct{}) *EnvoyController {
	return &EnvoyController{
		wg:         wg,
		addrString: addrString,
		stopCh:     stopCh,
		inventory:  NewInventory(),
	}
}

/*
func testres(config cache.SnapshotCache) {
	clusters := make([]cache.Resource, 1)
	clusters[0] = MakeCluster("service_google", 1*time.Second)
	endpoints := make([]cache.Resource, 1)
	endpoints[0] = MakeEndpoint("service_google", "216.58.208.46", 443)
	routes := make([]cache.Resource, 1)
	routes[0] = MakeRoute("route_google", "service_google", "www.google.com")
	listeners := make([]cache.Resource, 1)
	listeners[0] = MakeHTTPListener("listener_0", "0.0.0.0", "route_google", 10000)
	snap := cache.NewSnapshot("x", endpoints, clusters, routes, listeners)
	config.SetSnapshot("test-id", snap)
}
*/

func (e *EnvoyController) Start() {
	defer e.wg.Done()
	e.wg.Add(1)

	e.cb = &callbacks{}

	e.config = cache.NewSnapshotCache(true, Hasher{}, Logger{})
	srv := server.NewServer(e.config, e.cb)
	//	ctx := context.Background()

	glog.V(2).Info("Creating gRPC Server")
	grpcServer := grpc.NewServer()

	glog.V(2).Info("Creating gRPC listen port")
	lis, err := net.Listen("tcp", e.addrString)
	if err != nil {
		glog.Errorf("Failed to open xDS listen Port: %v", err)
		return
	}

	glog.Info("Registering envoy api to gRPC")
	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, srv)
	v2.RegisterEndpointDiscoveryServiceServer(grpcServer, srv)
	v2.RegisterClusterDiscoveryServiceServer(grpcServer, srv)
	v2.RegisterRouteDiscoveryServiceServer(grpcServer, srv)
	v2.RegisterListenerDiscoveryServiceServer(grpcServer, srv)

	glog.Info("xDS Management Server Listening")
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			glog.Errorf("gRPC Server Error: %v", err)
		}
	}()
	<-e.stopCh

	grpcServer.GracefulStop()
}
