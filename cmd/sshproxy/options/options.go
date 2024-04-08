package options

import (
	componentbaseconfig "k8s.io/component-base/config"
)

type Options struct {
	ClientConnection       componentbaseconfig.ClientConnectionConfiguration
	Master                 string
	Kubeconfig             string
	HostKeySecretNamespace string
	HostKeySecretName      string
}
