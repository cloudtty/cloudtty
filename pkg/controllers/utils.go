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
	cloudshellv1alpha2 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// IsJobFinished checks whether the given Job has finished execution.
// It does not discriminate between successful and failed terminations.
func IsJobFinished(job *batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}

	return false, ""
}

// IsCloudshellFinished checks whether the given cloudshell has finished execution.
// It does not discriminate between successful and failed terminations.
func IsCloudshellFinished(cloudsehll *cloudshellv1alpha2.CloudShell) bool {
	if cloudsehll.Status.Phase == cloudshellv1alpha2.PhaseCompleted ||
		cloudsehll.Status.Phase == cloudshellv1alpha2.PhaseFailed {
		return true
	}
	return false
}

func IsCloudShellReady(cloudshell *cloudshellv1alpha2.CloudShell) bool {
	return cloudshell.Status.Phase == cloudshellv1alpha2.PhaseReady
}
