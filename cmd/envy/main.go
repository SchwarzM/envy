package main

import (
	"os"

	"github.com/schwarzm/envy/internal/endpoints"

	"github.com/schwarzm/envy/internal/logger"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/schwarzm/envy/internal/envoy"
	"github.com/schwarzm/envy/internal/service"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	mainLog := log.Log.WithName("main")
	mainLog.Info("starting up")
	intLogger := logger.Logger{Log: mainLog}
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})

	if err != nil {
		mainLog.Error(err, "error creating manager")
		os.Exit(1)
	}
	inventory := cache.NewSnapshotCache(false, cache.IDHash{}, &intLogger)
	//TODO: this needs initial resources else everything will be down once this restarts
	snap := cache.NewSnapshot("0", nil, nil, nil, nil, nil, nil)
	err = inventory.SetSnapshot("lb", snap)
	if err != nil {
		mainLog.Error(err, "error setting initial snapshot")
		os.Exit(1)
	}
	envo := envoy.Controller{
		Log:    log.Log.WithName("envoy-controller"),
		Client: mgr.GetClient(),
		Inv:    inventory,
	}
	err = mgr.Add(&envo)
	if err != nil {
		mainLog.Error(err, "unable to add envoy controller")
		os.Exit(1)
	}

	//Service controller
	c, err := controller.New("service-controller", mgr, controller.Options{
		Reconciler: &service.Controller{
			Client: mgr.GetClient(),
		},
	})
	if err != nil {
		mainLog.Error(err, "error creating service controller")
		os.Exit(1)
	}
	if err = c.Watch(&source.Kind{Type: &core.Service{}}, &handler.EnqueueRequestForObject{}); err != nil {
		mainLog.Error(err, "unable to watch services")
		os.Exit(1)
	}

	//Endpoints controller
	e, err := controller.New("endpoints-controller", mgr, controller.Options{
		Reconciler: &endpoints.Controller{
			Client: mgr.GetClient(),
			Inv:    inventory,
		},
	})
	if err != nil {
		mainLog.Error(err, "error creating endpoints controller")
		os.Exit(1)
	}
	if err = e.Watch(&source.Kind{Type: &core.Endpoints{}}, &handler.EnqueueRequestForObject{}); err != nil {
		mainLog.Error(err, "unable to watch endpoints")
		os.Exit(1)
	}

	mainLog.Info("starting manager")
	sigs := signals.SetupSignalHandler()
	if err = mgr.Start(sigs); err != nil {
		mainLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	mainLog.Info("shutting down")
	envo.Server.GracefulStop()
}
