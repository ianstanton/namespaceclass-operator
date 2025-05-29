package controller

import (
	"time"

	"k8s.io/apimachinery/pkg/util/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	stantonshv1 "stanton.sh/namespace-class/api/v1"
)

var _ = Describe("NamespaceclassController", func() {
	var namespaceClass *stantonshv1.NamespaceClass

	Describe("NamespaceClass", func() {
		Context("When creating a namespaceclass", func() {
			It("Should create a namespaceclass object", func() {
				namespaceClass = &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-namespaceclass",
						Namespace: "default",
					},
					Spec: stantonshv1.NamespaceClassSpec{
						Resources: []runtime.RawExtension{},
					},
				}
				Expect(namespaceClass).NotTo(BeNil())
				Expect(namespaceClass.Name).To(Equal("test-namespaceclass"))
			})
		})
		Context("When creating a Namespace with a valid NamespaceClass label", func() {
			It("Should create resources defined in the NamespaceClass", func(ctx SpecContext) {
				nsclass := &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "public-network",
					},
					Spec: stantonshv1.NamespaceClassSpec{
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
				// Create Namespace with the label 'namespaceclass.stanton.sh/name: public-network'
				namespace := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-app",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "public-network",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
				// Check if the NetworkPolicy is created
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "my-app", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
			})
		})
		Context("When switching NamespaceClass", func() {
			It("Should apply the new resources and clean up the old", func(ctx SpecContext) {
				// Create a new NamespaceClass
				nsclass := &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "private-network",
					},
					Spec: stantonshv1.NamespaceClassSpec{
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
				nsclass2 := &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "image-pull-secret",
					},
					Spec: stantonshv1.NamespaceClassSpec{
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

				// Create a Namespace with the label 'namespaceclass.stanton.sh/name: private-network'
				namespace := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "your-app",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "private-network",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())

				// Check if the NetworkPolicy is created
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "deny-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				// Fetch the latest version of the Namespace
				latestNamespace := &corev1.Namespace{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, latestNamespace)).Should(Succeed())

				// Update the label on the latest version
				latestNamespace.Labels["namespaceclass.stanton.sh/name"] = "image-pull-secret"
				Expect(k8sClient.Update(ctx, latestNamespace)).Should(Succeed())

				// Check if the Secret is created
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "image-pull-secret"}, &corev1.Secret{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				// Check if the old NetworkPolicy is deleted
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "your-app", Name: "deny-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Not(Succeed()))
			})
		})
		Context("When updating a NamespaceClass", func() {
			It("Should apply the new resources and clean up the old for each namespace using the NamespaceClass", func(ctx SpecContext) {
				// Create a new NamespaceClass
				nsclass := &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "data-team",
					},
					Spec: stantonshv1.NamespaceClassSpec{
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
										PodSelector: metav1.LabelSelector{},
										Ingress:     []networkingv1.NetworkPolicyIngressRule{},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, nsclass)).Should(Succeed())

				// Create two Namespaces with the label 'namespaceclass.stanton.sh/name: public-network'
				namespace1 := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace1",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "data-team",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace1)).Should(Succeed())

				namespace2 := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace2",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "data-team",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace2)).Should(Succeed())

				// Check both namespaces have the NetworkPolicy defined in the NamespaceClass
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				// Update the NamespaceClass to add a secret and a ConfigMap
				nsclass.Spec.Resources = append(nsclass.Spec.Resources, runtime.RawExtension{
					Object: &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Secret",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "image-pull-secret",
						},
					},
				}, runtime.RawExtension{
					Object: &corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							Kind:       "ConfigMap",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "app-config",
						},
					},
				})
				Expect(k8sClient.Update(ctx, nsclass)).Should(Succeed())

				// Check both namespaces have the NetworkPolicy, Secret, and ConfigMap
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "image-pull-secret"}, &corev1.Secret{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "app-config"}, &corev1.ConfigMap{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "image-pull-secret"}, &corev1.Secret{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "app-config"}, &corev1.ConfigMap{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				// Update the NamespaceClass to remove the Secret
				nsclass.Spec.Resources = []runtime.RawExtension{
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
								PodSelector: metav1.LabelSelector{},
								Ingress:     []networkingv1.NetworkPolicyIngressRule{},
							},
						},
					},
					{
						Object: &corev1.ConfigMap{
							TypeMeta: metav1.TypeMeta{
								Kind:       "ConfigMap",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: "app-config",
							},
						},
					},
				}
				Expect(k8sClient.Update(ctx, nsclass)).Should(Succeed())

				// Check both namespaces have the NetworkPolicy and ConfigMap but not the Secret
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "image-pull-secret"}, &corev1.Secret{})
				}, 5*time.Second, 100*time.Millisecond).Should(Not(Succeed()))
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace1", Name: "app-config"}, &corev1.ConfigMap{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "allow-all-ingress"}, &networkingv1.NetworkPolicy{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "image-pull-secret"}, &corev1.Secret{})
				}, 5*time.Second, 100*time.Millisecond).Should(Not(Succeed()))
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "namespace2", Name: "app-config"}, &corev1.ConfigMap{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
			})
			It("Should update existing resources in all namespaces using the NamespaceClass", func(ctx SpecContext) {
				// Create a NamespaceClass with a Deployment
				deployment := &appsv1.Deployment{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-cd-server",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(1),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "argo-cd-server",
							},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "argo-cd-server",
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "argo-cd-server",
										Image: "argoproj/argocd-server:v2.0.0",
										Ports: []corev1.ContainerPort{
											{
												Name:          "server",
												ContainerPort: 8080,
												Protocol:      corev1.ProtocolTCP,
											},
										},
									},
								},
							},
						},
					},
				}

				rawDeployment, err := json.Marshal(deployment)
				Expect(err).NotTo(HaveOccurred())

				nsclass := &stantonshv1.NamespaceClass{
					TypeMeta: metav1.TypeMeta{
						Kind:       "NamespaceClass",
						APIVersion: "stanton.sh/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-cd",
					},
					Spec: stantonshv1.NamespaceClassSpec{
						Resources: []runtime.RawExtension{
							{
								Raw: rawDeployment,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, nsclass)).Should(Succeed())

				// Create a Namespace with the label 'namespaceclass.stanton.sh/name: infra-team'
				namespace1 := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-cd1",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "argo-cd",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace1)).Should(Succeed())

				// Create a second Namespace
				namespace2 := &corev1.Namespace{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "argo-cd2",
						Labels: map[string]string{
							"namespaceclass.stanton.sh/name": "argo-cd",
						},
					},
				}
				Expect(k8sClient.Create(ctx, namespace2)).Should(Succeed())

				// Check both namespaces have the Deployment defined in the NamespaceClass
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd1", Name: "argo-cd-server"}, &appsv1.Deployment{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd2", Name: "argo-cd-server"}, &appsv1.Deployment{})
				}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

				// Check the Deployment replicas are set to 1 in both namespaces
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd1", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(1)))
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd2", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(1)))

				// Update the NamespaceClass to change the Deployment replicas to 3
				var updatedDeployment appsv1.Deployment
				err = json.Unmarshal(nsclass.Spec.Resources[0].Raw, &updatedDeployment)
				Expect(err).NotTo(HaveOccurred())

				updatedDeployment.Spec.Replicas = int32Ptr(3)
				updatedRawDeployment, err := json.Marshal(updatedDeployment)
				Expect(err).NotTo(HaveOccurred())

				nsclass.Spec.Resources[0].Raw = updatedRawDeployment
				Expect(k8sClient.Update(ctx, nsclass)).Should(Succeed())

				// Check both namespaces have the updated Deployment replicas
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd1", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(3)))
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd2", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(3)))

				// Update back to 1 replica
				updatedDeployment.Spec.Replicas = int32Ptr(1)
				updatedRawDeployment, err = json.Marshal(updatedDeployment)
				Expect(err).NotTo(HaveOccurred())
				nsclass.Spec.Resources[0].Raw = updatedRawDeployment
				Expect(k8sClient.Update(ctx, nsclass)).Should(Succeed())

				// Check both namespaces have the updated Deployment replicas
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd1", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(1)))
				Eventually(func() int32 {
					deployment := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "argo-cd2", Name: "argo-cd-server"}, deployment)
					if err != nil {
						return 0
					}
					return *deployment.Spec.Replicas
				}, 5*time.Second, 100*time.Millisecond).Should(Equal(int32(1)))
			})
		})
	})
})

// int32Ptr is a helper function to convert an int to a pointer to int32 for Deployment replicas
func int32Ptr(i int) *int32 {
	i32 := int32(i)
	return &i32
}
