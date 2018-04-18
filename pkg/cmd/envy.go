package cmd

import (
	goflag "flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	ec "github.com/schwarzm/envy/pkg/endpoint-controller"
	env "github.com/schwarzm/envy/pkg/envoy-controller"
	sc "github.com/schwarzm/envy/pkg/service-controller"
)

type EnvyConfig struct {
	//bgpNeighbor    string
	//bgpRouterId    string
	envoyxDSAddr   string
	kubeconfigPath string
}

var EnvyCfg = EnvyConfig{}
var VERSION string

var EnvyCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		goflag.CommandLine.Parse([]string{})
		glog.Infof("Starting envy, version: %s", VERSION)

		// Create waitgroup
		wg := &sync.WaitGroup{}

		// Stop signal channel
		stopCh := make(chan struct{})

		// Create envoy config
		glog.V(2).Info("Creating envoy config store")
		// Start envoy xDS
		glog.Info("Starting envoy xDS server")
		envoycontroller := env.NewEnvoyController(wg, EnvyCfg.envoyxDSAddr, stopCh)
		go envoycontroller.Start()

		// Start bgp controller
		//glog.Info("Starting BGP controller")

		// Create kubernetes client
		glog.V(2).Info("Creating kubernetes client")
		config, err := clientcmd.BuildConfigFromFlags("", EnvyCfg.kubeconfigPath)
		if err != nil {
			glog.Fatalf("Unable to create kubernetes config: %v", err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			glog.Fatalf("Unable to create kubernetes client: %v", err)
		}
		version, err := clientset.Discovery().ServerVersion()
		if err != nil {
			glog.Fatalf("Unable to discover kubernetes version: %v", err)
		}
		glog.V(2).Infof("Discovered Kubernetes Version: %v", version)

		// OS Signal channel
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

		// Start service controller
		glog.Info("Starting Service controller")
		go sc.StartServiceController(clientset, envoycontroller, stopCh, wg)
		// Start endpoint controller
		glog.Info("Starting Endpoint controller")
		go ec.StartEndpointController(clientset, envoycontroller, stopCh, wg)

		<-sigs
		glog.Info("Shutting Down")
		close(stopCh)
		wg.Wait()
		glog.Info("Shutdown completed")
	},
}

func Execute() {
	if err := EnvyCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func InitFlags() {
	EnvyCmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
	//EnvyCmd.PersistentFlags().StringVar(&EnvyCfg.bgpNeighbor, "bgp_neighbor", "", "BGP Neighbor")
	//EnvyCmd.PersistentFlags().StringVar(&EnvyCfg.bgpRouterId, "bgp_routerid", "", "BGP Router ID")
	EnvyCmd.PersistentFlags().StringVar(&EnvyCfg.envoyxDSAddr, "envoy_xds_addr", ":18000", "Address for envoy xDS Server to listen to")
	EnvyCmd.PersistentFlags().StringVar(&EnvyCfg.kubeconfigPath, "kubeconfigpath", "", "Path to kubeconfig")
	EnvyCmd.PersistentFlags().Set("logtostderr", "true")
	EnvyCmd.PersistentFlags().MarkHidden("logtostderr")
	EnvyCmd.PersistentFlags().MarkHidden("log_backtrace_at")
	EnvyCmd.PersistentFlags().MarkHidden("stderrthreshold")
	EnvyCmd.PersistentFlags().MarkHidden("vmodule")
}
