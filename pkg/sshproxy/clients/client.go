package clients

import (
	"context"
	"fmt"

	"github.com/cloudtty/cloudtty/cmd/sshproxy/options"
	"github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	"github.com/cloudtty/cloudtty/pkg/generated/clientset/versioned"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const CloudProxyUserAgent = "cloudtty-cloudproxy"

type clients struct {
	globalClient     *clientset.Clientset
	cloudshellClient *versioned.Clientset
	opt              *options.Options
}

var _ Clients = (*clients)(nil)

type Clients interface {
	GetGlobalKubeClient() kubernetes.Interface
	GetCloudshellClient() versioned.Interface
	GetOptions() *options.Options
}

var cli *clients

func GetClients() Clients {
	return cli
}

func (c *clients) GetGlobalKubeClient() kubernetes.Interface {
	return c.globalClient
}

func (c *clients) GetCloudshellClient() versioned.Interface {
	return c.cloudshellClient
}

func (c *clients) GetOptions() *options.Options {
	return c.opt
}

func InitClient(o *options.Options) error {
	var err error
	var restConfig *rest.Config
	if len(o.Kubeconfig) == 0 {
		restConfig, err = clientconfig.GetConfig()
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags(o.Master, o.Kubeconfig)
	}
	if err != nil {
		klog.Errorf("build restConfig err: %v", err)
		return err
	}
	client, err := clientset.NewForConfig(rest.AddUserAgent(restConfig, CloudProxyUserAgent))
	if err != nil {
		klog.Errorf("build kube client err: %v, restConfig: %s", restConfig)
		return err
	}
	cloudshellClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("build cloudshell client err: %v, restConfig: %s", restConfig)
		return err
	}

	cli = &clients{
		globalClient:     client,
		cloudshellClient: cloudshellClient,
		opt:              o,
	}
	return nil
}

func GetClientsetByProxy(cloudProxy *v1alpha1.CloudProxy) (*kubernetes.Clientset, error) {
	kubeClient := GetClients().GetGlobalKubeClient()
	kubeconfSecret, err := kubeClient.CoreV1().Secrets(cloudProxy.Spec.KubeconRef.Namespace).Get(context.TODO(), cloudProxy.Spec.KubeconRef.Name, v1.GetOptions{})
	if err != nil {
		klog.Errorf("get kubeconfig secret err: %v, secret name: %s, namespace: %s", err, cloudProxy.Spec.KubeconRef.Name, cloudProxy.Spec.KubeconRef.Namespace)
		return nil, err
	}

	if len(kubeconfSecret.Data) == 0 {
		return nil, fmt.Errorf("kubeconfig secret data is empty, secret name: %s, namespace: %s", cloudProxy.Spec.KubeconRef.Name, cloudProxy.Spec.KubeconRef.Namespace)
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfSecret.Data["kubeconfig"])
	if err != nil {
		klog.Errorf("create cluster client err: %v, cluster: %s", err, cloudProxy.Name)
		return nil, err
	}

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func GetClientConfigByProxy(cloudProxy *v1alpha1.CloudProxy) (*rest.Config, error) {
	kubeClient := GetClients().GetGlobalKubeClient()
	kubeconfSecret, err := kubeClient.CoreV1().Secrets(cloudProxy.Spec.KubeconRef.Namespace).Get(context.TODO(), cloudProxy.Spec.KubeconRef.Name, v1.GetOptions{})
	if err != nil {
		klog.Errorf("get kubeconfig secret err: %v, secret name: %s, namespace: %s", err, cloudProxy.Spec.KubeconRef.Name, cloudProxy.Spec.KubeconRef.Namespace)
		return nil, err
	}

	if len(kubeconfSecret.Data) == 0 {
		return nil, fmt.Errorf("kubeconfig secret data is empty, secret name: %s, namespace: %s", cloudProxy.Spec.KubeconRef.Name, cloudProxy.Spec.KubeconRef.Namespace)
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfSecret.Data["kubeconfig"])
	if err != nil {
		klog.Errorf("create cluster client err: %v, cluster: %s", err, cloudProxy.Name)
		return nil, err
	}

	return clientConfig.ClientConfig()
}
