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
		Context("When switching NamespaceClass", func() {
			It("Should apply the new resources and clean up the old", func(ctx SpecContext) {
				// Create a new NamespaceClass
				nsclass := &akuityiov1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "akuity.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "private-network",
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
										Name: "deny-all-ingress",
									},
									Spec: networkingv1.NetworkPolicySpec{
										PodSelector: metav1.LabelSelector{},
										Ingress:     []networkingv1.NetworkPolicyIngressRule{},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, nsclass)).Should(Succeed())

				// Create a second NamespaceClass that has a secret for pulling images
				nsclass2 := &akuityiov1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "akuity.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "image-pull-secret",
					},
					Spec: akuityiov1.NamespaceClassSpec{
						Resources: []runtime.RawExtension{
							{
								Object: &corev1.Secret{
									TypeMeta: metav1.TypeMeta{
										Kind:       "Secret",
										APIVersion: "v1",
									},
									ObjectMeta: metav1.ObjectMeta{
										Name: "image-pull-secret",
									},
									Type: corev1.SecretTypeDockerConfigJson,
									Data: map[string][]byte{
										".dockerconfigjson": []byte(`{"auths":{"your-registry-url":{"username":"your-username","password":"your-password","email":"your-email","auth":"your-auth"}}}`),
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, nsclass2)).Should(Succeed())

				// Create a Namespace with the label 'namespaceclass.akuity.io/name: private-network'
				namespace := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "your-app",
						Labels: map[string]string{
							"namespaceclass.akuity.io/name": "private-network",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())

				// Wait for the controller to reconcile, sleep for now
				time.Sleep(1 * time.Second)

				// Check if the NetworkPolicy is created
				networkPolicy := &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "deny-all-ingress",
					},
				}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "deny-all-ingress"}, networkPolicy)).Should(Succeed())

				// Fetch the latest version of the Namespace
				latestNamespace := &corev1.Namespace{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, latestNamespace)).Should(Succeed())

				// Update the label on the latest version
				latestNamespace.Labels["namespaceclass.akuity.io/name"] = "image-pull-secret"
				Expect(k8sClient.Update(ctx, latestNamespace)).Should(Succeed())

				// Wait for the controller to reconcile, sleep for now
				time.Sleep(2 * time.Second)

				// Check if the Secret is created
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "image-pull-secret",
					},
				}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "image-pull-secret"}, secret)).Should(Succeed())

				// Check if the old NetworkPolicy is deleted
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "deny-all-ingress"}, networkPolicy)).Should(Not(Succeed()))
			})
		})
	})
})
