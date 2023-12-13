package helper

import (
	"encoding/base64"
	"net"

	cloudshellv1alpha2 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
)

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

func NewVirtualServiceTemplateValue(objectKey types.NamespacedName, vsConfig *cloudshellv1alpha2.VirtualServiceConfig, serviceName, routePath string) VirtualServiceTemplateValue {
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
