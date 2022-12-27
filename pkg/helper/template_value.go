package helper

import (
	"encoding/base64"
	"fmt"
	"net"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

type PodTemplateValue struct {
	Namespace string
	Name      string
	Command   string
	Secret    string
	Once      bool
	UrlArg    bool
	Ttl       int32
}

func NewPodTemplateValue(cloudShell *cloudshellv1alpha1.CloudShell) PodTemplateValue {
	return PodTemplateValue{
		Namespace: cloudShell.Namespace,
		Name:      fmt.Sprintf("cloudshell-%s", cloudShell.Name),
		Once:      cloudShell.Spec.Once,
		Secret:    cloudShell.Spec.SecretRef.Name,
		Command:   cloudShell.Spec.CommandAction,
		Ttl:       cloudShell.Spec.Ttl,
		UrlArg:    cloudShell.Spec.UrlArg,
	}
}

type ServiceTemplateValue struct {
	Name      string
	Namespace string
	JobName   string
	Type      string
}

func NewServiceTemplateValue(cloudShell *cloudshellv1alpha1.CloudShell, serviceType cloudshellv1alpha1.ExposureMode) ServiceTemplateValue {
	return ServiceTemplateValue{
		Name:      fmt.Sprintf("cloudshell-%s", cloudShell.Name),
		Namespace: cloudShell.Namespace,
		JobName:   fmt.Sprintf("cloudshell-%s", cloudShell.Name),
		Type:      string(serviceType),
	}
}

type IngressTemplateValue struct {
	Name             string
	Namespace        string
	IngressClassName string
	Path             string
	ServiceName      string
}

func NewIngressTemplateValue(objectKey types.NamespacedName, ingressClassName, serviceName, routePath string) IngressTemplateValue {
	return IngressTemplateValue{
		Name:             objectKey.Name,
		Namespace:        objectKey.Namespace,
		IngressClassName: ingressClassName,
		ServiceName:      serviceName,
		Path:             routePath,
	}
}

type VirtualServiceTemplateValue struct {
	Name        string
	Namespace   string
	ExportTo    string
	Gateway     string
	Path        string
	ServiceName string
}

func NewVirtualServiceTemplateValue(objectKey types.NamespacedName, vsConfig *cloudshellv1alpha1.VirtualServiceConfig, serviceName, routePath string) VirtualServiceTemplateValue {
	return VirtualServiceTemplateValue{
		Name:        objectKey.Name,
		Namespace:   objectKey.Namespace,
		ExportTo:    vsConfig.ExportTo,
		Gateway:     vsConfig.Gateway,
		Path:        routePath,
		ServiceName: serviceName,
	}
}

type KubeConfigTemplateValue struct {
	CAData string
	Server string
	Token  string
}

func NewKubeConfigTemplateValue(host, port, token string, caRaw []byte) KubeConfigTemplateValue {
	return KubeConfigTemplateValue{
		Server: "https://" + net.JoinHostPort(host, port),
		CAData: base64.StdEncoding.EncodeToString(caRaw),
		Token:  token,
	}
}
