package controller

import (
	"context"
	"fmt"
	"io/ioutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	destIPKey = "DEST_IP"
)

func parseFile(fn string, o interface{}) error {
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, o); err != nil {
		return err
	}
	return nil
}

type object interface {
	runtime.Object
	client.Object
}

func (r *ServiceReconciler) createOrUpdate(ctx context.Context, owner, obj object, fn func() error) error {
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		if err := controllerutil.SetOwnerReference(owner, obj, r.Scheme()); err != nil {
			return err
		}
		return fn()
	}); err != nil {
		return err
	}
	return nil
}

func (r *ServiceReconciler) reconcileService(ctx context.Context, svc *corev1.Service) error {
	stsSvc := &corev1.Service{}
	if err := parseFile("svc.yaml", stsSvc); err != nil {
		return err
	}

	svcOrig := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tailscale", svc.Name),
			Namespace: svc.Namespace,
			Labels:    make(map[string]string),
		},
	}
	return r.createOrUpdate(ctx, svc, svcOrig, func() error {
		svcOrig.Labels["svc"] = svc.Name
		stsSvc.Spec.Selector["svc"] = svc.Name
		stsSvc.Spec.DeepCopyInto(&svcOrig.Spec)
		return nil
	})
}

func (r *ServiceReconciler) reconcileStatefulSet(ctx context.Context, svc *corev1.Service) error {
	sts := &appsv1.StatefulSet{}
	if err := parseFile("sts.yaml", sts); err != nil {
		return err
	}

	addOrUpdateEnv(destIPKey, svc.Spec.ClusterIP, &sts.Spec.Template.Spec.Containers[0])

	stsOrig := &appsv1.StatefulSet{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tailscale", svc.Name),
			Namespace: svc.Namespace,
			Labels:    make(map[string]string),
		},
	}
	sts.Spec.Template.Spec.Hostname = hostName(svc)
	sts.Spec.Template.Labels["svc"] = svc.Name
	sts.Spec.Selector.MatchLabels["svc"] = svc.Name
	sts.Spec.ServiceName = sts.Name

	return r.createOrUpdate(ctx, svc, stsOrig, func() error {
		stsOrig.Labels["svc"] = svc.Name
		sts.Spec.DeepCopyInto(&stsOrig.Spec)
		return nil
	})
}

func addOrUpdateEnv(key, value string, c *corev1.Container) {
	for i := range c.Env {
		e := &c.Env[i]
		if e.Name != key {
			continue
		}
		e.Value = value
		e.ValueFrom = nil
		return
	}
	c.Env = append(c.Env, corev1.EnvVar{Name: key, Value: value})
}
