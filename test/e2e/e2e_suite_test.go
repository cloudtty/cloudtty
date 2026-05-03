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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ctx    context.Context
	cancel context.CancelFunc
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudTTY E2E Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())

	By("Checking that required tools are available")
	Expect(commandExists("kind")).To(BeTrue(), "kind is required for e2e tests")
	Expect(commandExists("helm")).To(BeTrue(), "helm is required for e2e tests")
	Expect(commandExists("docker")).To(BeTrue(), "docker is required for e2e tests")
	Expect(commandExists("kubectl")).To(BeTrue(), "kubectl is required for e2e tests")

	By("Ensuring kind cluster is running")
	Expect(isKindClusterRunning()).To(BeTrue(), "kind cluster must be running. Create one with: kind create cluster --name kind --config test/e2e/kind-config.yaml")

	By("Building operator image")
	buildDockerImage(operatorImage, "docker/operator/Dockerfile")

	By("Building cloudshell image")
	buildDockerImage(cloudshellImage, "docker/cloudshell/Dockerfile")

	By("Loading images into kind cluster")
	loadImageToKind(operatorImage)
	loadImageToKind(cloudshellImage)

	By("Installing cloudtty operator via Helm")
	installOperator()

	By("Initializing Kubernetes client")
	initK8sClient()

	By("Waiting for operator deployment to be ready")
	Eventually(func() bool {
		return isDeploymentReady(operatorNamespace, operatorDeployment)
	}, 3*time.Minute, 5*time.Second).Should(BeTrue())
})

var _ = AfterSuite(func() {
	cancel()
	By("Uninstalling cloudtty operator")
	uninstallOperator()
})

// handleTestFailure collects diagnostic information when a test fails.
var _ = JustAfterEach(func() {
	if CurrentSpecReport().Failed() {
		specName := CurrentSpecReport().FullText()
		fmt.Fprintf(GinkgoWriter, "\n=== TEST FAILED: %s ===\n", specName)
		fmt.Fprintf(GinkgoWriter, "--- Operator logs (last 100 lines) ---\n")
		// Try to get operator pod logs
		podList := &corev1.PodList{}
		if err := k8sClient.List(ctx, podList, client.InNamespace(operatorNamespace), client.MatchingLabels{"control-plane": operatorDeployment}); err == nil && len(podList.Items) > 0 {
			logs := getPodLogs(operatorNamespace, podList.Items[0].Name, operatorDeployment)
			lines := strings.Split(logs, "\n")
			start := 0
			if len(lines) > 100 {
				start = len(lines) - 100
			}
			for _, line := range lines[start:] {
				fmt.Fprintln(GinkgoWriter, line)
			}
		}
		fmt.Fprintf(GinkgoWriter, "=====================================\n\n")
	}
})
