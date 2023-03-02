/*
Copyright 2022.

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

package controllers

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"

	"github.com/cloudtty/cloudtty/pkg/utils/feature"
)

const (
	// AllowSecretStoreKubeconfig is a feature gate for the cloudshell to store kubeconfig in secret.
	//
	// owner: @calvin0327
	// alpha: v0.3.0
	// beta: v0.4.0
	AllowSecretStoreKubeconfig featuregate.Feature = "AllowSecretStoreKubeconfig"
)

func init() {
	runtime.Must(feature.MutableFeatureGate.Add(defaultFeatureGates))
}

// defaultFeatureGates consists of all known cloudtty-specific feature keys.
// To add a new feature, define a key for it above and add it here.
var defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	AllowSecretStoreKubeconfig: {Default: true, PreRelease: featuregate.Beta},
}
