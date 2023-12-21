package v1alpha1

import (
	"github.com/cloudtty/cloudtty/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&CloudShell{}, func(obj interface{}) { SetObjectDefaultsCloudShell(obj.(*CloudShell)) })
	return nil
}

// SetObjectDefaultsCloudShell set defaults for cloudshell
func SetObjectDefaultsCloudShell(in *CloudShell) {
	setDefaultsCloudShell(in)
}

func setDefaultsCloudShell(obj *CloudShell) {
	if len(obj.Spec.CommandAction) == 0 {
		obj.Spec.CommandAction = "bash"
	}

	if obj.Spec.TTLSecondsAfterStarted == nil {
		obj.Spec.TTLSecondsAfterStarted = pointer.Int64(3600)
	}

	if len(obj.Spec.ExposeMode) == 0 {
		obj.Spec.ExposeMode = ExposureServiceNodePort
	}

	if len(obj.Spec.Image) == 0 {
		obj.Spec.Image = constants.DefaultTtydImage
	}
}
