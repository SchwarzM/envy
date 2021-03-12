package envoy

import (
	"context"
	"net"
	"sync"

	"google.golang.org/grpc/reflection"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"

	"github.com/go-logr/logr"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller struct {
	Log    logr.Logger
	Inv    cache.SnapshotCache
	Client client.Client
	Server *grpc.Server
}

func (c *Controller) Start(ctx context.Context) error {
	cbv3 := Callbacks{
		Signal: make(chan struct{}),
		Debug:  true,
		mu:     sync.Mutex{},
	}
	srv3 := server.NewServer(ctx, c.Inv, &cbv3)
	c.Server = grpc.NewServer()
	lis, err := net.Listen("tcp", ":9991")
	if err != nil {
		c.Log.Error(err, "error creating listen socket")
		return err
	}
	defer lis.Close()

	clusterservice.RegisterClusterDiscoveryServiceServer(c.Server, srv3)
	endpointservice.RegisterEndpointDiscoveryServiceServer(c.Server, srv3)
	listenerservice.RegisterListenerDiscoveryServiceServer(c.Server, srv3)
	reflection.Register(c.Server)
	if err = c.Server.Serve(lis); err != nil {
		c.Log.Error(err, "error starting grpc server")
		return err
	}
	return nil
}
