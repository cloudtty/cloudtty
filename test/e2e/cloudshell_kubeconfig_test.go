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

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
)

var _ = Describe("CloudShell Kubeconfig", Ordered, func() {
	var testNS string

	BeforeAll(func() {
		By("Creating test namespace with unique name")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-test-kubeconfig-",
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		testNS = ns.Name

		DeferCleanup(func() {
			By("Cleaning up test namespace")
			deleteNamespaceIgnoreNotFound(ctx, testNS)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNS}, &corev1.Namespace{})
				return apierrors.IsNotFound(err)
			}, 2*time.Minute, 2*time.Second).Should(BeTrue())
		})
	})

	It("should auto-generate kubeconfig Secret when SecretRef is not provided", func() {
		cloudShellName := "test-auto-kubeconfig"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR without SecretRef")
		cloudShell := &cloudshellv1alpha1.CloudShell{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudShellName,
				Namespace: testNS,
			},
			Spec: cloudshellv1alpha1.CloudShellSpec{
				CommandAction: "bash",
				ExposeMode:    cloudshellv1alpha1.ExposureServiceNodePort,
			},
		}
		Expect(k8sClient.Create(ctx, cloudShell)).To(Succeed())

		DeferCleanup(func() {
			By("Deleting CloudShell CR")
			cs := &cloudshellv1alpha1.CloudShell{}
			if err := k8sClient.Get(ctx, cloudShellKey, cs); err == nil {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cs))).To(Succeed())
			}
		})

		By("Waiting for CloudShell to be Ready and SecretRef to be populated")
		var secretName string
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
			g.Expect(cs.Spec.SecretRef).NotTo(BeNil())
			g.Expect(cs.Spec.SecretRef.Name).NotTo(BeEmpty())
			secretName = cs.Spec.SecretRef.Name
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying the auto-generated Secret exists and contains kubeconfig data")
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: testNS}, secret)).To(Succeed())
		Expect(secret.Data).To(HaveKey("config"))
		Expect(secret.Data["config"]).NotTo(BeEmpty())
	})

	It("should remain not-ready when referencing a non-existent Secret", func() {
		cloudShellName := "test-missing-secret"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR referencing a non-existent Secret")
		cloudShell := &cloudshellv1alpha1.CloudShell{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudShellName,
				Namespace: testNS,
			},
			Spec: cloudshellv1alpha1.CloudShellSpec{
				CommandAction: "bash",
				ExposeMode:    cloudshellv1alpha1.ExposureServiceNodePort,
				SecretRef: &cloudshellv1alpha1.LocalSecretReference{
					Name: "non-existent-secret",
				},
			},
		}
		Expect(k8sClient.Create(ctx, cloudShell)).To(Succeed())

		DeferCleanup(func() {
			By("Deleting CloudShell CR")
			cs := &cloudshellv1alpha1.CloudShell{}
			if err := k8sClient.Get(ctx, cloudShellKey, cs); err == nil {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, cs))).To(Succeed())
			}
		})

		By("Ensuring CloudShell does not become Ready within 30s")
		Consistently(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).NotTo(Equal(cloudshellv1alpha1.PhaseReady))
		}, 30*time.Second, 5*time.Second).Should(Succeed())
	})
})
