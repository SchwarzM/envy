package service

import (
	"context"
	"fmt"

	"github.com/schwarzm/envy/internal/envy"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Controller struct {
	Client client.Client
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	svc := &core.Service{}
	err := c.Client.Get(ctx, req.NamespacedName, svc)
	if errors.IsNotFound(err) {
		log.Error(err, "could not find service")
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not fetch service: %+v", err)
	}
	log.Info("reconciling service")
	if svc.Spec.Type != "LoadBalancer" {
		log.V(10).Info("service not of type LoadBalancer: ignoring")
		return ctrl.Result{}, nil
	}
	value, ok := svc.ObjectMeta.Labels[envy.ServiceLabel]
	if !ok {
		log.V(10).Info("service has no envy label: ignoring")
		return ctrl.Result{}, nil
	}
	log.Info("label found", "label", value)
	// at this point we know the state of the world has changed
	if len(svc.Spec.LoadBalancerIP) == 0 {
		workingCopy := svc.DeepCopy()
		workingCopy.Spec.LoadBalancerIP = value
		err = c.Client.Update(ctx, workingCopy)
		if err != nil {
			log.Error(err, "error assigning lb ip")
			return ctrl.Result{}, err
		}
		workingCopy.Status.LoadBalancer.Ingress = []core.LoadBalancerIngress{{IP: value}}
		err = c.Client.Status().Update(ctx, workingCopy)
		if err != nil {
			log.Error(err, "error updating status")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
