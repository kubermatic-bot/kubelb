package controllers

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubelbiov1alpha1 "manager/pkg/api/v1alpha1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *GlobalLoadBalancerReconciler) reconcileService(desiredService *corev1.Service) error {
	log := r.Log.WithValues("globalloadbalancer", "l4-svc")

	actualService := &corev1.Service{}
	err := r.Get(r.ctx, types.NamespacedName{
		Name:      desiredService.Name,
		Namespace: desiredService.Namespace,
	}, actualService)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating service", "namespace", desiredService.Namespace, "name", desiredService.Name)
		return r.Create(r.ctx, desiredService)
	}

	if !reflect.DeepEqual(desiredService.Spec.Ports, actualService.Spec.Ports) {
		log.Info("Updating service", "namespace", desiredService.Namespace, "name", desiredService.Name)
		actualService.Spec.Ports = desiredService.Spec.Ports

		return r.Update(r.ctx, actualService)

	}

	return nil
}

func (r *GlobalLoadBalancerReconciler) reconcileEndpoints(desiredEndpoints *corev1.Endpoints) error {

	log := r.Log.WithValues("globalloadbalancer", "l4-endpoints")

	actualEndpoints := &corev1.Endpoints{}
	err := r.Get(r.ctx, types.NamespacedName{
		Name:      desiredEndpoints.Name,
		Namespace: desiredEndpoints.Namespace,
	}, actualEndpoints)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		log.Info("Creating endpoints", "namespace", desiredEndpoints.Namespace, "name", desiredEndpoints.Name)
		return r.Create(r.ctx, desiredEndpoints)
	}

	if !reflect.DeepEqual(desiredEndpoints.Subsets, actualEndpoints.Subsets) {
		log.Info("Updating endpoints", "namespace", desiredEndpoints.Namespace, "name", desiredEndpoints.Name)
		actualEndpoints.Subsets = desiredEndpoints.Subsets

		return r.Update(r.ctx, actualEndpoints)

	}

	return nil
}

func (r *GlobalLoadBalancerReconciler) handleL4(glb *kubelbiov1alpha1.GlobalLoadBalancer) error {

	log := r.Log.WithValues("globalloadbalancer", "l4")

	desiredService := r.mapService(glb)

	if err := ctrl.SetControllerReference(glb, desiredService, r.Scheme); err != nil {
		log.Error(err, "Unable to set controller reference")
		return err
	}

	err := r.reconcileService(desiredService)

	if err != nil {
		log.Error(err, "Unable to reconcile service")
		return err
	}

	desiredEndpoints := r.mapEndpoints(glb)

	if err = ctrl.SetControllerReference(glb, desiredEndpoints, r.Scheme); err != nil {
		return err
	}

	err = r.reconcileEndpoints(desiredEndpoints)

	if err != nil {
		log.Error(err, "Unable to reconcile endpoints")
		return err
	}

	return nil

}

func (r *GlobalLoadBalancerReconciler) mapService(glb *kubelbiov1alpha1.GlobalLoadBalancer) *corev1.Service {

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: glb.Spec.Ports,
			Type:  corev1.ServiceTypeLoadBalancer,
		},
	}
}

func (r *GlobalLoadBalancerReconciler) mapEndpoints(glb *kubelbiov1alpha1.GlobalLoadBalancer) *corev1.Endpoints {

	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glb.Name,
			Namespace: glb.Namespace,
		},
		Subsets: glb.Spec.Subsets,
	}
}
