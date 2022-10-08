package constants

const (
	DefaultPathPrefix           = "/apis/v1alpha1/cloudshell"
	DefaultIngressName          = "cloudshell-ingress"
	DefaultVirtualServiceName   = "cloudshell-virtualService"
	DefaultServicePort          = 7681
	DefauletWebttyContainerName = "web-tty"

	CloudshellOwnerLabelKey = "cloudshell.cloudtty.io/owner-name"

	JobTemplatePath = "/etc/cloudtty/job-temp.yaml"
)
