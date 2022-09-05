package gclient

import (
	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// aggregatedScheme aggregates Kubernetes and extended schemes.
var aggregatedScheme = runtime.NewScheme()

func init() {
	_ = scheme.AddToScheme(aggregatedScheme)
	utilruntime.Must(batchv1.AddToScheme(aggregatedScheme))
	utilruntime.Must(corev1.AddToScheme(aggregatedScheme))
	utilruntime.Must(cloudshellv1alpha1.AddToScheme(aggregatedScheme))
	utilruntime.Must(istionetworkingv1beta1.AddToScheme(aggregatedScheme))
}

// NewSchema returns a singleton schema set which aggregated Kubernetes's schemes and extended schemes.
func NewSchema() *runtime.Scheme {
	return aggregatedScheme
}
