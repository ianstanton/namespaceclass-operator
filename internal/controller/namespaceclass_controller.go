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
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
	Scheme          *runtime.Scheme
	DiscoveryClient *discovery.DiscoveryClient
}

// NewDecoder is used for deserializing RawExtension objects from the
// NamespaceClassSpec.Resources field.
func NewDecoder(scheme *runtime.Scheme) runtime.Decoder {
	codecs := serializer.NewCodecFactory(scheme)
	return codecs.UniversalDeserializer()
}

const (
	NamespaceClassLabel             = "namespaceclass.akuity.io/name"
	CurrentNamespaceClassAnnotation = "namespaceclass.akuity.io/current-namespace-class"
)

// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=akuity.io,resources=namespaceclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NamespaceClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Watch for changes to Namespace resources and check for the presence of the
	// "namespaceclass.akuity.io/name" label.
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); client.IgnoreNotFound(err) != nil {
		log.Error(err, "unable to fetch Namespace")
		return ctrl.Result{}, err
	} else if err != nil {
		log.Info("Namespace not found, ignoring")
		return ctrl.Result{}, nil
	}

	// Check if the Namespace is being deleted
	if namespace.DeletionTimestamp != nil {
		log.Info("Namespace is being deleted, skipping")
		return ctrl.Result{}, nil
	}

	// Check if the Namespace has the "namespaceclass.akuity.io/name" label
	if !r.namespaceHasLabel(namespace) {
		return ctrl.Result{}, nil
	}

	// Reconcile the NamespaceClass resources
	namespaceClassName := namespace.Labels[NamespaceClassLabel]
	if err := r.reconcileNamespaceClass(ctx, namespace, namespaceClassName, &log); err != nil {
		log.Error(err, "failed to reconcile NamespaceClass", "namespaceClassName", namespaceClassName)
		return ctrl.Result{}, err
	}
	log.Info("Successfully reconciled NamespaceClass", "namespaceClassName", namespaceClassName)

	return ctrl.Result{}, nil
}

// namespaceHasLabel checks if the Namespace has the "namespaceclass.akuity.io/name" label
func (r *NamespaceClassReconciler) namespaceHasLabel(namespace *corev1.Namespace) bool {
	_, ok := namespace.Labels[NamespaceClassLabel]
	return ok
}

// reconcileNamespaceClass reconciles the NamespaceClass resources for the given Namespace
func (r *NamespaceClassReconciler) reconcileNamespaceClass(ctx context.Context, namespace *corev1.Namespace, namespaceClassName string, log *logr.Logger) error {
	// Get the current NamespaceClass
	namespaceClass := &akuityiov1.NamespaceClass{}
	if err := r.Get(ctx, client.ObjectKey{Name: namespaceClassName}, namespaceClass); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("NamespaceClass not found, skipping reconciliation", "namespaceClassName", namespaceClassName)
			return nil
		}
	}

	// Update namespace annotations if not present
	if err := r.updateNamespaceAnnotations(ctx, namespace, namespaceClass); err != nil {
		return err
	}

	// If the current-namespace-class annotation does not match the NamespaceClass, clean up
	if currentClass, exists := namespace.Annotations[CurrentNamespaceClassAnnotation]; exists && currentClass != namespaceClass.Name {
		// Cleanup resources that are not part of the current NamespaceClass
		if err := r.cleanupResources(ctx, namespace, currentClass); err != nil {
			return err
		}
		// Update the annotation to the new NamespaceClass
		namespace.Annotations[CurrentNamespaceClassAnnotation] = namespaceClass.Name
		if err := r.Update(ctx, namespace); err != nil {
			return err
		}
	}

	// Get the current resources in the namespace that are managed by the NamespaceClass
	currentResources, err := r.getCurrentResources(ctx, namespace, namespaceClass.Name)
	if err != nil {
		log.Error(err, "failed to get current resources", "namespace", namespace.Name)
		return err
	}

	// Create or update resources
	if err := r.createOrUpdateResource(ctx, namespaceClass, *namespace, log); err != nil {
		return err
	}

	// Delete unused resources
	if err := r.deleteUnusedResources(ctx, namespace, namespaceClass, currentResources); err != nil {
		return err
	}

	return nil
}

// updateNamespaceAnnotations updates the namespace annotations with the current NamespaceClass
func (r *NamespaceClassReconciler) updateNamespaceAnnotations(ctx context.Context, namespace *corev1.Namespace, namespaceClass *akuityiov1.NamespaceClass) error {
	if namespace.Annotations == nil {
		namespace.Annotations = make(map[string]string)
	}
	if _, exists := namespace.Annotations[CurrentNamespaceClassAnnotation]; !exists {
		namespace.Annotations[CurrentNamespaceClassAnnotation] = namespaceClass.Name
		// Update the namespace with the new annotation
		if err := r.Update(ctx, namespace); err != nil {
			return err
		}
	}
	return nil
}

// createOrUpdateResource creates or updates the resource in the namespace
func (r *NamespaceClassReconciler) createOrUpdateResource(ctx context.Context, namespaceClass *akuityiov1.NamespaceClass, namespace corev1.Namespace, log *logr.Logger) error {
	decoder := NewDecoder(r.Scheme)
	for _, resource := range namespaceClass.Spec.Resources {
		log.Info("Reconciling resource", "resource", resource)
		u := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode(resource.Raw, nil, u)
		if err != nil {
			log.Error(err, "failed to decode resource", "resource", resource)
			continue
		}

		u.SetGroupVersionKind(*gvk)
		u.SetNamespace(namespace.Name)
		u.SetName(u.GetName())

		// Check if the resource already exists
		current := u.DeepCopy()
		err = r.Get(ctx, client.ObjectKey{Namespace: namespace.Name, Name: u.GetName()}, current)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get resource", "resource", resource)
			return err
		}

		if apierrors.IsNotFound(err) {
			// Resource does not exist, so we create it
			u.SetAnnotations(map[string]string{NamespaceClassLabel: namespaceClass.Name})
			ownerRef := metav1.OwnerReference{
				APIVersion: akuityiov1.GroupVersion.String(),
				Kind:       "NamespaceClass",
				Name:       namespaceClass.Name,
				UID:        namespaceClass.UID,
				Controller: func() *bool { b := true; return &b }(),
			}
			u.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
			err = r.Create(ctx, u)
			if err != nil {
				log.Error(err, "failed to create resource", "resource", resource)
				return err
			}
		} else {
			// Resource already exists, so we update it
			patch := client.MergeFrom(current.DeepCopy())
			current.Object["spec"] = u.Object["spec"]
			current.SetAnnotations(map[string]string{NamespaceClassLabel: namespaceClass.Name})
			ownerRef := metav1.OwnerReference{
				APIVersion: akuityiov1.GroupVersion.String(),
				Kind:       "NamespaceClass",
				Name:       namespaceClass.Name,
				UID:        namespaceClass.UID,
				Controller: func() *bool { b := true; return &b }(),
			}
			current.SetOwnerReferences([]metav1.OwnerReference{ownerRef})
			err = r.Patch(ctx, current, patch)
			if err != nil {
				log.Error(err, "failed to patch resource", "resource", resource)
				return err
			}
		}
	}
	return nil
}

// getCurrentResources gets the current resources in the namespace that are managed by the NamespaceClass
func (r *NamespaceClassReconciler) getCurrentResources(ctx context.Context, namespace *corev1.Namespace, namespaceClassName string) ([]string, error) {
	// Set up DiscoveryClient for getting arbitrary resource types
	discoveryClient := r.DiscoveryClient
	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	var currentResources []string
	for _, resourceList := range resources {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, resource := range resourceList.APIResources {
			if !resource.Namespaced {
				continue
			}

			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    resource.Kind,
			}

			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(gvk)

			err = r.List(ctx, list, client.InNamespace(namespace.Name))
			if err != nil {
				continue
			}

			for _, item := range list.Items {
				annotations := item.GetAnnotations()
				if annotations != nil {
					// Check if the resource is managed by the NamespaceClass
					if value, ok := annotations[NamespaceClassLabel]; ok && value == namespaceClassName {
						currentResources = append(currentResources, item.GetKind()+"/"+item.GetName())
					}
				}
			}
		}
	}

	return currentResources, nil
}

// deleteUnusedResources deletes resources in the namespace that are not defined in the NamespaceClass
func (r *NamespaceClassReconciler) deleteUnusedResources(ctx context.Context, namespace *corev1.Namespace, namespaceClass *akuityiov1.NamespaceClass, currentResources []string) error {
	// Get the resources defined in the NamespaceClass
	desiredResources := make(map[string]bool)
	decoder := NewDecoder(r.Scheme)
	for _, resource := range namespaceClass.Spec.Resources {
		obj, _, err := decoder.Decode(resource.Raw, nil, nil)
		if err != nil {
			return err
		}
		desiredResources = addResourceToMap(desiredResources, obj)
	}

	// Delete resources that are no longer defined in the NamespaceClass
	for _, resource := range currentResources {
		if !desiredResources[resource] {
			parts := strings.Split(resource, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid resource string: %s", resource)
			}
			resourceParts := strings.Split(parts[0], ".")
			gvk := schema.GroupVersionKind{}
			if len(resourceParts) == 1 {
				gvk.Kind = resourceParts[0]
			} else {
				gvk.Group = strings.Join(resourceParts[:len(resourceParts)-1], ".")
				gvk.Kind = resourceParts[len(resourceParts)-1]
			}

			// Get the GVK for the resource
			discoveryClient := r.DiscoveryClient
			resources, err := discoveryClient.ServerPreferredResources()
			if err != nil {
				return err
			}
			for _, resourceList := range resources {
				gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
				if err != nil {
					return err
				}
				for _, apiResource := range resourceList.APIResources {
					if apiResource.Kind == gvk.Kind {
						gvk.Group = gv.Group
						gvk.Version = gv.Version
						break
					}
				}
			}

			// Delete the resource
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(gvk)
			obj.SetNamespace(namespace.Name)
			obj.SetName(parts[1])
			err = r.Delete(ctx, obj)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// addResourceToMap adds a resource to a map of desired resources
func addResourceToMap(resources map[string]bool, obj runtime.Object) map[string]bool {
	resources[obj.GetObjectKind().GroupVersionKind().Kind+"/"+obj.(metav1.Object).GetName()] = true
	return resources
}

// cleanupResources cleans up the resources in the namespace that have an annotation NamespaceClassLabel that does not match the
// current NamespaceClass parameter
func (r *NamespaceClassReconciler) cleanupResources(ctx context.Context, namespace *corev1.Namespace, currentClass string) error {
	// Set up DiscoveryClient for getting arbitrary resource types
	discoveryClient := r.DiscoveryClient
	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return err
	}

	for _, resourceList := range resources {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return err
		}

		for _, resource := range resourceList.APIResources {
			if !resource.Namespaced {
				continue
			}

			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    resource.Kind,
			}

			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(gvk)

			err = r.List(ctx, list, client.InNamespace(namespace.Name))
			if err != nil {
				continue
			}

			for _, item := range list.Items {
				annotations := item.GetAnnotations()
				if annotations != nil {
					// Check if the resource was created under the old NamespaceClass
					if value, ok := annotations["namespaceclass.akuity.io/name"]; ok && value == currentClass {
						err = r.Delete(ctx, &item)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&akuityiov1.NamespaceClass{}).
		Watches(&corev1.Namespace{}, &handler.EnqueueRequestForObject{}).
		Watches(
			&akuityiov1.NamespaceClass{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				log := logf.FromContext(ctx)
				log.Info("NamespaceClass changed, enqueueing namespaces", "namespaceclass", obj.GetName())
				namespaceList := &corev1.NamespaceList{}
				if err := r.List(ctx, namespaceList); err != nil {
					log.Error(err, "failed to list namespaces")
					return nil
				}

				var requests []reconcile.Request
				for _, namespace := range namespaceList.Items {
					if namespace.Labels[NamespaceClassLabel] == obj.GetName() {
						log.Info("Enqueueing namespace", "namespace", namespace.Name)
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: "",
								Name:      namespace.Name,
							},
						})
					}
				}
				return requests
			}),
		).
		Named("namespaceclass").
		Complete(r)
}
