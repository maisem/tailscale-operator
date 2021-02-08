package controller

import (
	"context"
	"fmt"
	"strings"

	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	enableAnnotation   = "tailscale.maisem.dev/enable"
	ipAnnotation       = "tailscale.maisem.dev/ip"
	deviceIDAnnotation = "tailscale.maisem.dev/deviceID"
	finalizer          = "tailscale.maisem.dev/finalizer"
)

func New(manager controllers.Manager) error {
	return controllers.
		NewControllerManagedBy(manager). // Create the Controller
		For(&corev1.Service{}).          // ReplicaSet is the Application API
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(&ServiceReconciler{Client: manager.GetClient()})
}

func hostName(svc *corev1.Service) string {
	return fmt.Sprintf("%s-%s", svc.Name, string(svc.UID)[:8])
}

// ServiceReconciler is a simple Controller example implementation.
type ServiceReconciler struct {
	client.Client
}

func (a *ServiceReconciler) handleDelete(ctx context.Context, svc *corev1.Service) (controllers.Result, error) {
	if !controllerutil.ContainsFinalizer(svc, finalizer) {
		return controllers.Result{}, nil
	}
	// TODO: On disable delete the statefulset.
	dID, ok := svc.Annotations[deviceIDAnnotation]
	if !ok {
		return controllers.Result{}, nil
	}
	_ = dID
	// TODO: Delete from tailscale.
	controllerutil.RemoveFinalizer(svc, finalizer)
	if err := a.Update(ctx, svc); err != nil {
		return controllers.Result{}, err
	}
	return controllers.Result{}, nil
}

func (a *ServiceReconciler) deployTailscale(ctx context.Context, svc *corev1.Service) error {
	if err := a.reconcileService(ctx, svc); err != nil {
		return err
	}
	return a.reconcileStatefulSet(ctx, svc)
}

func (a *ServiceReconciler) Reconcile(ctx context.Context, req controllers.Request) (controllers.Result, error) {
	svc := &corev1.Service{}
	if err := a.Get(ctx, req.NamespacedName, svc); err != nil {
		return controllers.Result{}, err
	}

	if ant, ok := svc.Annotations[enableAnnotation]; !ok || strings.ToLower(ant) != "true" || !svc.GetDeletionTimestamp().IsZero() {
		return a.handleDelete(ctx, svc)
	}

	controllerutil.AddFinalizer(svc, finalizer)
	if err := a.Update(ctx, svc); err != nil {
		return controllers.Result{}, err
	}

	if err := a.deployTailscale(ctx, svc); err != nil {
		return controllers.Result{}, err
	}

	// TODO: annotate with ip address & device ID.

	return controllers.Result{}, nil
}
