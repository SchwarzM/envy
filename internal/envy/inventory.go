package envy

import (
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/schwarzm/envy/internal/logger"
)

const TypeURL = "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment"

type Inventory struct {
	EDS     *cache.LinearCache
	Default cache.SnapshotCache
	Mux     cache.MuxCache
}

func NewInventory(log logger.Logger) *Inventory {
	var inv Inventory
	inv.EDS = cache.NewLinearCache(TypeURL)
	inv.EDS.UpdateResource("lb", nil)
	defaultCache := cache.NewSnapshotCache(false, cache.IDHash{}, &log)
	snap := cache.NewSnapshot("0", nil, nil, nil, nil, nil, nil)
	defaultCache.SetSnapshot("lb", snap)
	inv.Default = defaultCache
	inv.Mux = cache.MuxCache{
		Classify: func(request cache.Request) string {
			if request.TypeUrl == TypeURL {
				return "eds"
			} else {
				return "default"
			}
		},
		Caches: map[string]cache.Cache{
			"eds":     inv.EDS,
			"default": inv.Default,
		},
	}
	return &inv
}
