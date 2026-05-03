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
)

var _ = Describe("CloudTTY Operator Deployment", func() {
	It("should have the controller manager deployment running", func() {
		By("Checking the operator deployment exists and is ready")
		Expect(isDeploymentReady(operatorNamespace, operatorDeployment)).To(BeTrue())
	})

	It("should have the controller manager pods running", func() {
		By("Checking operator pods are in Running state")
		Eventually(func() bool {
			return isPodRunning(operatorNamespace, map[string]string{"control-plane": operatorDeployment})
		}, 2*time.Minute, 3*time.Second).Should(BeTrue())
	})

	// Note: The current version of cloudtty operator does not expose a metrics endpoint
	// via controller-runtime (it uses a custom leader-election and controller bootstrap).
	// If metrics are added in the future, this test can be enabled.
	// It("should serve metrics on the metrics endpoint", func() { ... })
})
