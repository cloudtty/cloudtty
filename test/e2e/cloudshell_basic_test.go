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
	"fmt"
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

var _ = Describe("CloudShell Basic Lifecycle", Ordered, func() {
	var (
		testNS         string
		cloudShellName = "test-cloudshell"
		cloudShellKey  types.NamespacedName
		serviceName    string
	)

	BeforeAll(func() {
		By("Creating test namespace with unique name")
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "e2e-test-basic-",
			},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		testNS = ns.Name

		DeferCleanup(func() {
			By("Cleaning up test namespace")
			deleteNamespaceIgnoreNotFound(ctx, testNS)
			// Wait for namespace to be fully deleted to avoid conflicts in subsequent runs
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: testNS}, &corev1.Namespace{})
				return apierrors.IsNotFound(err)
			}, 2*time.Minute, 2*time.Second).Should(BeTrue())
		})
	})

	It("should create a CloudShell and reconcile successfully", func() {
		cloudShellKey = types.NamespacedName{Name: cloudShellName, Namespace: testNS}
		serviceName = fmt.Sprintf("cloudshell-%s", cloudShellName)

		By("Creating a CloudShell CR with NodePort exposure")
		cloudShell := &cloudshellv1alpha1.CloudShell{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudShellName,
				Namespace: testNS,
			},
			Spec: cloudshellv1alpha1.CloudShellSpec{
				CommandAction:          "bash",
				ExposeMode:             cloudshellv1alpha1.ExposureServiceNodePort,
				TTLSecondsAfterStarted: int64Ptr(3600),
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

		By("Waiting for CloudShell status to become Ready")
		Eventually(func(g Gomega) {
			cs := &cloudshellv1alpha1.CloudShell{}
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cs.Status.Phase).To(Equal(cloudshellv1alpha1.PhaseReady))
			g.Expect(cs.Status.AccessURL).NotTo(BeEmpty())
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying Worker Pod is created and running")
		Eventually(func(g Gomega) {
			podList := &corev1.PodList{}
			err := k8sClient.List(ctx, podList,
				client.InNamespace(testNS),
				client.MatchingLabels{constants.WorkerOwnerLabelKey: cloudShellName},
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(podList.Items).To(HaveLen(1))
			g.Expect(podList.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
			// Ensure container is ready
			containerReady := false
			for _, cond := range podList.Items[0].Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
					containerReady = true
					break
				}
			}
			g.Expect(containerReady).To(BeTrue())
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("Verifying Service is created with correct type and labels")
		svc := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: testNS}, svc)
		}, 30*time.Second, 2*time.Second).Should(Succeed())

		Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Labels).To(HaveKeyWithValue(constants.WorkerOwnerLabelKey, cloudShellName))

		By("Verifying the ttyd endpoint is accessible via port-forward")
		cmd, localPort, err := portForwardService(testNS, serviceName, 7681)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})

		Eventually(func() int {
			statusCode, _, err := httpGet(fmt.Sprintf("http://127.0.0.1:%d", localPort))
			if err != nil {
				return 0
			}
			return statusCode
		}, 30*time.Second, 2*time.Second).Should(Equal(200))
	})

	It("should clean up resources when CloudShell is deleted", func() {
		cloudShellKey = types.NamespacedName{Name: cloudShellName, Namespace: testNS}
		serviceName = fmt.Sprintf("cloudshell-%s", cloudShellName)

		By("Ensuring the CloudShell CR exists")
		cs := &cloudshellv1alpha1.CloudShell{}
		Expect(k8sClient.Get(ctx, cloudShellKey, cs)).To(Succeed())

		By("Deleting the CloudShell CR")
		Expect(k8sClient.Delete(ctx, cs)).To(Succeed())

		By("Waiting for CloudShell CR to be removed")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, cloudShellKey, cs)
			return apierrors.IsNotFound(err)
		}, 2*time.Minute, 3*time.Second).Should(BeTrue())

		By("Verifying associated Service is deleted")
		svc := &corev1.Service{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: testNS}, svc)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		By("Verifying Worker Pod is deleted")
		Eventually(func() int {
			podList := &corev1.PodList{}
			_ = k8sClient.List(ctx, podList,
				client.InNamespace(testNS),
				client.MatchingLabels{constants.WorkerOwnerLabelKey: cloudShellName},
			)
			return len(podList.Items)
		}, 2*time.Minute, 3*time.Second).Should(Equal(0))
	})
})

func int64Ptr(i int64) *int64 {
	return &i
}
