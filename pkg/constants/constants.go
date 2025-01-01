package constants

const (
	DefaultPathPrefix         = "/apis/v1alpha1/cloudshell"
	DefaultIngressName        = "cloudshell-ingress"
	DefaultVirtualServiceName = "cloudshell-virtualService"
	DefaultServicePort        = 7681
	DefaultTtydImage          = "ghcr.io/cloudtty/cloudshell:v0.8.2"

	CloudshellPodLabelKey = "cloudshell.cloudtty.io/pod-name"

	WorkerOwnerLabelKey        = "worker.cloudtty.io/owner-name"
	WorkerRequestLabelKey      = "worker.cloudtty.io/request"
	WorkerBindingCountLabelKey = "worker.cloudtty.io/binding-count"
	WorkerNameLabelKey         = "worker.cloudtty.io/name"

	PodTemplatePath = "/etc/cloudtty/pod-temp.yaml"
)
