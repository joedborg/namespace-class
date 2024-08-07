/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	npv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	namespaceclassv1alpha1 "github.com/joedborg/namespace-class/api/v1alpha1"
)

// NamespaceClassReconciler reconciles a NamespaceClass object
type NamespaceClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=namespaceclass.josephb.org,resources=namespaceclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=namespaceclass.josephb.org,resources=namespaceclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=namespaceclass.josephb.org,resources=namespaceclasses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NamespaceClass object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *NamespaceClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// Fetch the NamespaceClass instance
	var namespaceClass namespaceclassv1alpha1.NamespaceClass
	if err := r.Client.Get(ctx, req.NamespacedName, &namespaceClass); err != nil {
		l.Error(err, "Failed to get NamespaceClass")
		return ctrl.Result{}, err
	}

	// List namespaces with the specified label
	labelSelector := client.MatchingLabels{"namespaceclass.josephb.org/name": namespaceClass.Name}
	var namespaceList corev1.NamespaceList
	if err := r.Client.List(ctx, &namespaceList, labelSelector); err != nil {
		l.Error(err, "Failed to list namespaces")
		return ctrl.Result{}, err
	}

	for _, ns := range namespaceList.Items {
		// Create a NetworkPolicy object
		var networkPolicy npv1.NetworkPolicy
		networkPolicy.Name = fmt.Sprintf("%s-policy", namespaceClass.Name)
		networkPolicy.Namespace = ns.Name
		networkPolicy.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: map[string]string{"namespaceclass.josephb.org/name": namespaceClass.Name},
		}
		networkPolicy.Spec.PolicyTypes = []npv1.PolicyType{
			npv1.PolicyTypeIngress,
		}
		networkPolicy.Spec.Ingress = []npv1.NetworkPolicyIngressRule{
			{
				From: []npv1.NetworkPolicyPeer{
					{
						IPBlock: &npv1.IPBlock{
							CIDR: namespaceClass.Spec.AllowedIPs,
						},
					},
				},
			},
		}

		// Create the NetworkPolicy if it doesn't exist, otherwise update it
		if err := r.Client.Update(ctx, &networkPolicy); errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, &networkPolicy); err != nil {
				l.Error(err, "Failed to create NetworkPolicy")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			l.Error(err, "Failed to create NetworkPolicy")
			return ctrl.Result{}, err
		}

		// List pods in the namespace
		podSelector := client.InNamespace(ns.Name)
		var podList corev1.PodList
		if err := r.Client.List(ctx, &podList, podSelector); err != nil {
			l.Error(err, "Failed to list namespaces")
			return ctrl.Result{}, err
		}

		for _, pod := range podList.Items {
			// Add the label to the pod
			pod.Labels["namespaceclass.josephb.org/name"] = namespaceClass.Name
			if err := r.Client.Update(ctx, &pod); err != nil {
				l.Error(err, "Failed to update pod")
				return ctrl.Result{}, err
			}
		}

		l.Info(fmt.Sprintf("Created %s in %s", networkPolicy.Name, ns.Name))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&namespaceclassv1alpha1.NamespaceClass{}).
		Complete(r)
}
