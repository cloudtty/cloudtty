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
	"github.com/cloudtty/cloudtty/pkg/constants"
)

var _ = Describe("WorkerPool", Ordered, func() {
	var testNS string

	BeforeAll(func() {
		By("Creating test namespace with unique name")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-test-workerpool-",
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

	It("should create a worker pod when CloudShell is requested", func() {
		cloudShellName := "test-workerpool"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR")
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

		By("Waiting for CloudShell to be Ready")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying a worker pod was created in the same namespace")
		Eventually(func(g Gomega) {
			podList := &corev1.PodList{}
			err := k8sClient.List(ctx, podList,
				client.InNamespace(testNS),
				client.MatchingLabels{constants.WorkerOwnerLabelKey: cloudShellName},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(podList.Items).To(HaveLen(1))
			g.Expect(podList.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
			containerReady := false
			for _, cond := range podList.Items[0].Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					containerReady = true
					break
				}
			}
			g.Expect(containerReady).To(BeTrue())
		}, 2*time.Minute, 3*time.Second).Should(Succeed())
	})

	It("should reset worker label after CloudShell is deleted", func() {
		cloudShellName := "test-workerpool-reset"
		cloudShellKey := types.NamespacedName{Name: cloudShellName, Namespace: testNS}

		By("Creating a CloudShell CR")
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

		By("Waiting for CloudShell to be Ready")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Deleting the CloudShell CR")
		Expect(k8sClient.Delete(ctx, cloudShell)).To(Succeed())

		By("Waiting for CloudShell CR to be removed")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, cloudShellKey, &cloudshellv1alpha1.CloudShell{})
			return apierrors.IsNotFound(err)
		}, 2*time.Minute, 3*time.Second).Should(BeTrue())

		By("Verifying the worker pod owner label was reset to empty")
		Eventually(func(g Gomega) {
			podList := &corev1.PodList{}
			err := k8sClient.List(ctx, podList,
				client.InNamespace(testNS),
				client.MatchingLabels{constants.WorkerOwnerLabelKey: ""},
			)
			g.Expect(err).NotTo(HaveOccurred())
			// At least one idle worker should exist after reset
			g.Expect(len(podList.Items)).To(BeNumerically(">=", 1))
		}, 2*time.Minute, 3*time.Second).Should(Succeed())
	})
})
