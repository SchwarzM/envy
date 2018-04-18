package enpoint_controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	enc "github.com/schwarzm/envy/pkg/envoy-controller"
)

const ServiceLabel = "schwarzm/envy"

type QueueItem struct {
	key    string
	action string
}

type EndpointController struct {
	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller
	envoy    *enc.EnvoyController
}

func findLabel(labels map[string]string) bool {
	for label := range labels {
		if label == ServiceLabel {
			return true
		}
	}
	return false
}

func NewEndpointController(queue workqueue.RateLimitingInterface, indexer cache.Indexer, informer cache.Controller, envoy *enc.EnvoyController) *EndpointController {
	return &EndpointController{
		indexer:  indexer,
		queue:    queue,
		informer: informer,
		envoy:    envoy,
	}
}

func StartEndpointController(clientset *kubernetes.Clientset, envoy *enc.EnvoyController, stopCh chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	wg.Add(1)

	glog.Info("Creating Endpoint Controller")

	glog.V(2).Info("Creating Endpoint Watch")
	serviceWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "endpoints", "", fields.Everything())

	glog.V(2).Info("Creating Endpoint Work Queue")
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	glog.V(2).Info("Creating Endpoint Indexer")
	indexer, informer := cache.NewIndexerInformer(serviceWatcher, &v1.Endpoints{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				glog.V(10).Infof("Error adding Endpoint: %v", err)
				return
			}
			labels := obj.(*v1.Endpoints).GetLabels()
			if findLabel(labels) {
				glog.V(10).Infof("Adding Endpoint key: %v", key)
				qi := QueueItem{
					key:    key,
					action: "add",
				}
				queue.Add(qi)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(new)
			if err != nil {
				glog.V(10).Infof("Error updating Endpoint: %v", err)
				return
			}
			labels := new.(*v1.Endpoints).GetLabels()
			if findLabel(labels) {
				//the new item has the label
				glog.V(10).Infof("Update Endpoint key: %v", key)
				qi := QueueItem{
					key:    key,
					action: "add",
				}
				queue.Add(qi)
				return
			}
			labels = old.(*v1.Endpoints).GetLabels()
			if findLabel(labels) {
				//the old item has the label the new one not.
				glog.V(10).Infof("Delete Endpoint key: %v label was removed", key)
				qi := QueueItem{
					key:    key,
					action: "del",
				}
				queue.Add(qi)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				glog.V(10).Infof("Error updating Endpoint: %v", err)
				return
			}
			labels := obj.(*v1.Endpoints).GetLabels()
			if findLabel(labels) {
				glog.V(10).Infof("Delete Endpoint key: %v", key)
				qi := QueueItem{
					key:    key,
					action: "del",
				}
				queue.Add(qi)
			}
		},
	}, cache.Indexers{})

	glog.V(2).Info("Running Endpoint Controller")
	controller := NewEndpointController(queue, indexer, informer, envoy)
	controller.Run(stopCh)
}

func (ec *EndpointController) Run(stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer ec.queue.ShutDown()

	glog.V(2).Info("Starting Informer")
	go ec.informer.Run(stopCh)

	glog.V(2).Info("Warming Cache")
	if !cache.WaitForCacheSync(stopCh, ec.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	glog.V(2).Info("Starting Worker thread")
	go wait.Until(ec.runWorker, time.Second, stopCh)
	<-stopCh
	glog.Info("Stopping Service Controller")
}

func (ec *EndpointController) runWorker() {
	for ec.processNextItem() {
	}
}

func (ec *EndpointController) processNextItem() bool {
	// Wait until there is a new item in the working queue
	qi, quit := ec.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because two pods with the same key are never processed in
	// parallel.
	defer ec.queue.Done(qi)

	// Invoke the method containing the business logic
	err := ec.handle(qi.(QueueItem))
	// Handle the error if something went wrong during the execution of the business logic
	ec.handleErr(err, qi)
	return true
}

func (ec *EndpointController) handle(qi QueueItem) error {
	obj, exists, err := ec.indexer.GetByKey(qi.key)
	if err != nil {
		glog.Errorf("Fetching object with key %s from store failed with %v", qi.key, err)
		return err
	}

	if qi.action == "add" && !exists {
		return fmt.Errorf("Error handling %v no such item in the indexer", qi.key)
	}

	switch qi.action {
	case "add":
		err := ec.envoy.AddOrUpdateEndpoints(obj.(*v1.Endpoints), qi.key)
		return err
	case "del":
		err := ec.envoy.DeleteEndpoints(qi.key)
		return err
	default:
		glog.Errorf("Error handling %v, unknown action: %v", qi.key, qi.action)
		return fmt.Errorf("Error handling %v, unknown action: %v", qi.key, qi.action)
	}
}

func (ec *EndpointController) handleErr(err error, key interface{}) {
	if err == nil {
		ec.queue.Forget(key)
		return
	}

	if ec.queue.NumRequeues(key) < 5 {
		glog.Errorf("Error syncing service %v: %v", key, err)
		ec.queue.AddRateLimited(key)
		return
	}

	ec.queue.Forget(key)
	runtime.HandleError(err)
	glog.Errorf("Dropping service %q out of the queue: %v", key, err)
}
