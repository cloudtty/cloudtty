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
package options

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/cloudtty/cloudtty/cmd/app/config"
	"github.com/cloudtty/cloudtty/pkg/generated/clientset/versioned"
	"github.com/cloudtty/cloudtty/pkg/utils/gclient"
)

const (
	NamespaceCloudttySystem              = "cloudtty-system"
	CloudShellControllerManagerUserAgent = "cloudshell-controller-manager"
)

// Options contains everything necessary to create and run cloudshell-manager.
type Options struct {
	// LeaderElection defines the configuration of leader election client.
	LeaderElection   componentbaseconfig.LeaderElectionConfiguration
	ClientConnection componentbaseconfig.ClientConnectionConfiguration

	Master                     string
	Kubeconfig                 string
	CoreWorkerLimit            int
	MaxWorkerLimit             int
	ScaleInWorkerQueueDuration int
	ClouShellImage             string
	Logs                       *logs.Options
}

func NewOptions() (*Options, error) {
	var (
		leaderElection   componentbaseconfigv1alpha1.LeaderElectionConfiguration
		clientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration
	)
	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&leaderElection)
	componentbaseconfigv1alpha1.RecommendedDefaultClientConnectionConfiguration(&clientConnection)

	leaderElection.ResourceName = "cloudshell-controller-manager"
	leaderElection.ResourceNamespace = NamespaceCloudttySystem
	leaderElection.ResourceLock = resourcelock.LeasesResourceLock

	clientConnection.ContentType = runtime.ContentTypeJSON

	var options Options

	// not need scheme.Convert
	if err := componentbaseconfigv1alpha1.Convert_v1alpha1_LeaderElectionConfiguration_To_config_LeaderElectionConfiguration(&leaderElection, &options.LeaderElection, nil); err != nil {
		return nil, err
	}
	if err := componentbaseconfigv1alpha1.Convert_v1alpha1_ClientConnectionConfiguration_To_config_ClientConnectionConfiguration(&clientConnection, &options.ClientConnection, nil); err != nil {
		return nil, err
	}

	options.Logs = logs.NewOptions()
	return &options, nil
}

// initFlags initializes flags by section name.
func (o *Options) Flags() cliflag.NamedFlagSets {
	nfs := cliflag.NamedFlagSets{}
	genericfs := nfs.FlagSet("generic")

	genericfs.StringVar(&o.ClientConnection.ContentType, "kube-api-content-type", o.ClientConnection.ContentType, "Content type of requests sent to apiserver.")
	genericfs.Float32Var(&o.ClientConnection.QPS, "kube-api-qps", o.ClientConnection.QPS, "QPS to use while talking with kubernetes apiserver.")
	genericfs.Int32Var(&o.ClientConnection.Burst, "kube-api-burst", o.ClientConnection.Burst, "Burst to use while talking with kubernetes apiserver.")
	genericfs.IntVar(&o.CoreWorkerLimit, "core-worker-limit", 5, "The core limit of worker pool.")
	genericfs.IntVar(&o.MaxWorkerLimit, "max-worker-limit", 10, "The max limit of worker pool.")
	genericfs.IntVar(&o.ScaleInWorkerQueueDuration, "scale-in-worker-queue-duration", 180, "The duration (in minutes) to scale in the workers.")
	genericfs.StringVar(&o.ClouShellImage, "cloudshell-image", "", "The cloudshell image")

	fs := nfs.FlagSet("misc")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig).")
	fs.StringVar(&o.Kubeconfig, "kubeconfig", o.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")

	options.BindLeaderElectionFlags(&o.LeaderElection, genericfs)
	logsapi.AddFlags(o.Logs, nfs.FlagSet("logs"))

	return nfs
}

func (o *Options) Config() (*config.Config, error) {
	if filedErr := o.Validate(); filedErr.ToAggregate() != nil {
		return nil, filedErr.ToAggregate()
	}

	var err error
	var kubeconfig *rest.Config
	if len(o.Kubeconfig) == 0 {
		kubeconfig, err = clientconfig.GetConfig()
	} else {
		kubeconfig, err = clientcmd.BuildConfigFromFlags(o.Master, o.Kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	kubeconfig.ContentConfig.AcceptContentTypes = o.ClientConnection.AcceptContentTypes
	kubeconfig.ContentConfig.ContentType = o.ClientConnection.ContentType
	kubeconfig.QPS = o.ClientConnection.QPS
	kubeconfig.Burst = int(o.ClientConnection.Burst)

	client, err := clientset.NewForConfig(rest.AddUserAgent(kubeconfig, CloudShellControllerManagerUserAgent))
	if err != nil {
		return nil, err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: CloudShellControllerManagerUserAgent})

	runtimeClient, err := runtimeclient.New(kubeconfig, runtimeclient.Options{
		Scheme: gclient.NewSchema(),
	})
	if err != nil {
		return nil, err
	}

	cloudshellClient, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return &config.Config{
		KubeClient:                 client,
		CloudShellClient:           cloudshellClient,
		Client:                     runtimeClient,
		Kubeconfig:                 kubeconfig,
		EventRecorder:              eventRecorder,
		CoreWorkerLimit:            o.CoreWorkerLimit,
		MaxWorkerLimit:             o.MaxWorkerLimit,
		ScaleInWorkerQueueDuration: o.ScaleInWorkerQueueDuration,
		CloudShellImage:            o.ClouShellImage,

		LeaderElection: o.LeaderElection,
	}, nil
}
