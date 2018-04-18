package envoy_controller

import (
	"sync"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/golang/glog"
)

type callbacks struct {
	signal   chan struct{}
	fetches  int
	requests int
	mu       sync.Mutex
}

func (cb *callbacks) Report() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	glog.Info("Report: \n\tFetches: %v\n\tRequests: %v", cb.fetches, cb.requests)
}

func (cb *callbacks) OnStreamOpen(id int64, typ string) {
	glog.V(2).Infof("Stream %d open for %s", id, typ)
}

func (cb *callbacks) OnStreamClosed(id int64) {
	glog.V(2).Infof("Stream %d closed", id)
}

func (cb *callbacks) OnStreamRequest(int64, *v2.DiscoveryRequest) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.requests++
}

func (cb *callbacks) OnStreamResponse(int64, *v2.DiscoveryRequest, *v2.DiscoveryResponse) {}

func (cb *callbacks) OnFetchRequest(req *v2.DiscoveryRequest) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.fetches++
}

func (cb *callbacks) OnFetchResponse(*v2.DiscoveryRequest, *v2.DiscoveryResponse) {}
