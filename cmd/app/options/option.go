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
	"time"

	"github.com/cloudtty/cloudtty/pkg/utils/feature"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/component-base/config/options"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
)

const (
	defaultBindAddress      = "0.0.0.0"
	defaultPort             = 10357
	NamespaceCloudttySystem = "cloudtty-system"
)

var (
	// defualt 15/10/2
	defaultElectionLeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	defaultElectionRenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	defaultElectionRetryPeriod   = metav1.Duration{Duration: 2 * time.Second}
)

// Options contains everything necessary to create and run cloudshell-manager.
type Options struct {
	// LeaderElection defines the configuration of leader election client.
	LeaderElection *componentbaseconfig.LeaderElectionConfiguration
	// BindAddress is the IP address on which to listen for the --secure-port port.
	BindAddress string
	// SecurePort is the port that the the server serves at.
	// Note: We hope support https in the future once controller-runtime provides the functionality.
	SecurePort int
	// MetricsBindAddress is the TCP address that the controller should bind to
	// for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	// Defaults to ":8080".
	MetricsBindAddress string

	Logs *logs.Options

	// Flags hold the parsed CLI flags.
	Flags *cliflag.NamedFlagSets
}

// NewOptions builds an empty options.
func NewOptions() *Options {
	o := &Options{
		LeaderElection: &componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceNamespace: NamespaceCloudttySystem,
			ResourceName:      "cloudshell-controller-manager",
		},
		Logs: logs.NewOptions(),
	}
	o.initFlags()

	return o
}

// initFlags initializes flags by section name.
func (o *Options) initFlags() {
	if o.Flags != nil {
		return
	}

	nfs := cliflag.NamedFlagSets{}
	generic := nfs.FlagSet("generic")
	generic.StringVar(&o.BindAddress, "bind-address", defaultBindAddress, "The IP address on which to listen for the --secure-port port.")
	generic.IntVar(&o.SecurePort, "secure-port", defaultPort, "The secure port on which to serve HTTPS.")
	generic.StringVar(&o.MetricsBindAddress, "metrics-bind-address", ":8080", "The TCP address that the controller should bind to for serving prometheus metrics(e.g. 127.0.0.1:8088, :8088)")

	options.BindLeaderElectionFlags(o.LeaderElection, nfs.FlagSet("leader election"))

	feature.MutableFeatureGate.AddFlag(nfs.FlagSet("feature gate"))

	logsapi.AddFlags(o.Logs, nfs.FlagSet("logs"))
	o.Flags = &nfs
}
