package controller_test

import (
	akuityiov1 "akuity.io/namespace-class/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	})
})
