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

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
)

var _ = Describe("CloudShell TTL", Ordered, func() {
	var testNS string

	BeforeAll(func() {
		By("Creating test namespace with unique name")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-test-ttl-",
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

	It("should auto-delete CloudShell when TTL expires with cleanup=true", func() {
		cloudShellName := "test-ttl-cleanup"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR with 30s TTL and cleanup enabled")
		cloudShell := &cloudshellv1alpha1.CloudShell{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudShellName,
				Namespace: testNS,
			},
			Spec: cloudshellv1alpha1.CloudShellSpec{
				CommandAction:          "bash",
				ExposeMode:             cloudshellv1alpha1.ExposureServiceNodePort,
				TTLSecondsAfterStarted: int64Ptr(30),
				Cleanup:                true,
			},
		}
		Expect(k8sClient.Create(ctx, cloudShell)).To(Succeed())

		By("Waiting for CloudShell status to become Ready")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Waiting for TTL to expire and CloudShell CR to be automatically deleted")
		Eventually(func() bool {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			return apierrors.IsNotFound(err)
		}, 2*time.Minute, 3*time.Second).Should(BeTrue())
	})

	It("should set CloudShell phase to Complete when TTL expires without cleanup", func() {
		cloudShellName := "test-ttl-complete"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR with 30s TTL and cleanup disabled")
		cloudShell := &cloudshellv1alpha1.CloudShell{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudShellName,
				Namespace: testNS,
			},
			Spec: cloudshellv1alpha1.CloudShellSpec{
				CommandAction:          "bash",
				ExposeMode:             cloudshellv1alpha1.ExposureServiceNodePort,
				TTLSecondsAfterStarted: int64Ptr(30),
				Cleanup:                false,
			},
		}
		Expect(k8sClient.Create(ctx, cloudShell)).To(Succeed())

		By("Waiting for CloudShell status to become Ready")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Waiting for TTL to expire and CloudShell phase to become Complete")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseCompleted))
		}, 2*time.Minute, 3*time.Second).Should(Succeed())

		By("Cleaning up the completed CloudShell CR")
		Expect(k8sClient.Delete(ctx, cloudShell)).To(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, cloudShellKey, &cloudshellv1alpha1.CloudShell{})
			return apierrors.IsNotFound(err)
		}, 1*time.Minute, 2*time.Second).Should(BeTrue())
	})
})
