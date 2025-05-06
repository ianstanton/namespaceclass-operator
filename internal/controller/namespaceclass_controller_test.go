package controller

import (
	"time"

	akuityiov1 "akuity.io/namespace-class/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("NamespaceclassController", func() {
	var namespaceClass *akuityiov1.NamespaceClass

	Describe("NamespaceClass", func() {
		Context("When creating a namespaceclass", func() {
			It("Should create a namespaceclass object", func() {
				namespaceClass = &akuityiov1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "akuity.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-namespaceclass",
						Namespace: "default",
					},
					Spec: akuityiov1.NamespaceClassSpec{
						Resources: []runtime.RawExtension{},
					},
				}
				Expect(namespaceClass).NotTo(BeNil())
				Expect(namespaceClass.Name).To(Equal("test-namespaceclass"))
			})
		})
		Context("When creating a Namespace with a valid NamespaceClass label", func() {
			It("Should create resources defined in the NamespaceClass", func(ctx SpecContext) {
				nsclass := &akuityiov1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "akuity.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "public-network",
					},
					Spec: akuityiov1.NamespaceClassSpec{
						Resources: []runtime.RawExtension{
							{
								Object: &networkingv1.NetworkPolicy{
									TypeMeta: metav1.TypeMeta{
										Kind:       "NetworkPolicy",
										APIVersion: "networking.k8s.io/v1",
									},
									ObjectMeta: metav1.ObjectMeta{
										Name: "allow-all-ingress",
									},
									Spec: networkingv1.NetworkPolicySpec{
										PodSelector: metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "my-app",
											},
										},
										Ingress: []networkingv1.NetworkPolicyIngressRule{},
									},
								},
							},
						},
					},
				}
				// Create a NamespaceClass
				Expect(k8sClient.Create(ctx, nsclass)).Should(Succeed())
				// Create Namespace with the label 'namespaceclass.akuity.io/name: public-network'
				namespace := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-app",
						Labels: map[string]string{
							"namespaceclass.akuity.io/name": "public-network",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
				// Check if the NetworkPolicy is created
				networkPolicy := &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "allow-all-ingress",
					},
				}
				// Wait for the controller to reconcile, sleep for now
				time.Sleep(1 * time.Second)
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "my-app", Name: "allow-all-ingress"}, networkPolicy)).Should(Succeed())
			})
		})
	})
})
