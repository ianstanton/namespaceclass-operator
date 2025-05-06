/*
Copyright 2025.

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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	akuityiov1 "akuity.io/namespace-class/api/v1"
)

// NamespaceClassReconciler reconciles a NamespaceClass object
type NamespaceClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewDecoder is used for deserializing RawExtension objects from the
// NamespaceClassSpec.Resources field.
func NewDecoder(scheme *runtime.Scheme) runtime.Decoder {
	codecs := serializer.NewCodecFactory(scheme)
	return codecs.UniversalDeserializer()
}

const NamespaceClassLabel = "namespaceclass.akuity.io/name"

// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NamespaceClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// Watch for changes to Namespace resources and check for the presence of the
	// "namespaceclass.akuity.io/name" label.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); client.IgnoreNotFound(err) != nil {
		logf.FromContext(ctx).Error(err, "unable to fetch Namespace")
		return ctrl.Result{}, err
	} else if err != nil {
		logf.FromContext(ctx).Info("Namespace not found, ignoring")
		return ctrl.Result{}, nil
	}

	// Check if the namespace has the "namespaceclass.akuity.io/name" label
	labelValue, ok := namespace.Labels[NamespaceClassLabel]
	if !ok {
		// The namespace does not have the label, so we can ignore it
		return ctrl.Result{}, nil
	} else {
		// The namespace has the label, so for now we'll log the value
		logf.FromContext(ctx).Info("Namespace has a namespaceclass label", "labelValue", labelValue)

		// Check for the NamespaceClass resource with the same name as the label value
		namespaceClass := &akuityiov1.NamespaceClass{}

		// Log error if the NamespaceClass is not found
		if err := r.Get(ctx, client.ObjectKey{Name: labelValue}, namespaceClass); err != nil {
			logf.FromContext(ctx).Error(err, "cannot find NamespaceClass with name", "name", labelValue)
			return ctrl.Result{}, nil
		}

		// Log the NamespaceClass we found
		logf.FromContext(ctx).Info("Found NamespaceClass", "namespaceClass", namespaceClass)

		// Reconcile the resources defined in the NamespaceClass
		decoder := NewDecoder(r.Scheme)
		for _, resource := range namespaceClass.Spec.Resources {
			logf.FromContext(ctx).Info("Reconciling resource", "resource", resource)
			obj, _, err := decoder.Decode(resource.Raw, nil, nil)
			if err != nil {
				// Log error if deserializing fails
				logf.FromContext(ctx).Error(err, "failed to decode resource", "resource", resource)
				continue
			}
			// Create or update the resource in the namespace
			// Type assertion to implement client.Object interface
			typedObj, ok := obj.(client.Object)
			if !ok {
				logf.FromContext(ctx).Error(nil, "could not implement client.Object interface via type assertion", "resource", resource)
				continue
			}

			// Check if the resource already exists
			err = r.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: typedObj.GetName()}, typedObj)
			if err == nil {
				// Resource already exists, so we update it
				if err := r.Client.Update(ctx, typedObj); err != nil {
					logf.FromContext(ctx).Error(err, "failed to update resource", "resource", typedObj)
					return ctrl.Result{}, err
				}
				continue
			}

			_, err = ctrl.CreateOrUpdate(ctx, r.Client, typedObj, func() error {
				// Set the owner reference to the NamespaceClass
				if err := ctrl.SetControllerReference(namespaceClass, typedObj, r.Scheme); err != nil {
					return err
				}
				// Set the namespace to the one being reconciled
				typedObj.SetNamespace(namespace.Name)
				// Set the labels to include the NamespaceClass label
				labels := typedObj.GetLabels()
				if labels == nil {
					labels = make(map[string]string)
				}
				labels[NamespaceClassLabel] = namespaceClass.Name
				typedObj.SetLabels(labels)

				// Create or update the resource
				if err := r.Client.Create(ctx, typedObj); err != nil {
					if meta.IsNoMatchError(err) {
						logf.FromContext(ctx).Error(err, "resource type not supported", "resource", resource)
						return nil
					}
					logf.FromContext(ctx).Error(err, "failed to create resource", "resource", typedObj)
					return err
				}
				logf.FromContext(ctx).Info("Resource created", "resource", typedObj)

				return nil
			})
			if err != nil {
				logf.FromContext(ctx).Error(err, "failed to reconcile resource", "resource", typedObj)
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&akuityiov1.NamespaceClass{}).
		Watches(&corev1.Namespace{}, &handler.EnqueueRequestForObject{}).
		Named("namespaceclass").
		Complete(r)
}
