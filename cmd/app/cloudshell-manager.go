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
package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	"github.com/cloudtty/cloudtty/cmd/app/options"
	"github.com/cloudtty/cloudtty/pkg/constants"
	"github.com/cloudtty/cloudtty/pkg/controllers"
	"github.com/cloudtty/cloudtty/pkg/utils/feature"
	"github.com/cloudtty/cloudtty/pkg/utils/gclient"
	"github.com/cloudtty/cloudtty/pkg/version"
)

func init() {
	runtime.Must(logsv1.AddFeatureGates(feature.MutableFeatureGate))
}

// NewManagerCommand creates a *cobra.Command object with default parameters
func NewManagerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()
	cmd := &cobra.Command{
		Use:   "cloudshell-manager",
		Short: `Run this command in order to run cloudshell controller manager`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			// Activate logging as soon as possible, after that
			// show flags with the final logging configuration.
			if err := logsapi.ValidateAndApply(opts.Logs, feature.FeatureGate); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			cliflag.PrintFlags(cmd.Flags())

			// validate options
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}

			return Run(ctx, opts)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	nfs := opts.Flags
	verflag.AddFlags(nfs.FlagSet("global"))
	globalflag.AddGlobalFlags(nfs.FlagSet("global"), cmd.Name(), logs.SkipLoggingConfigurationFlags())
	fs := cmd.Flags()
	for _, f := range nfs.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, *nfs, cols)

	if err := cmd.MarkFlagFilename("config", "yaml", "yml", "json"); err != nil {
		klog.ErrorS(err, "Failed to mark flag filename")
	}

	return cmd
}

func Run(ctx context.Context, opts *options.Options) error {
	klog.Infof("cloudshell-controller-manager version: %s", version.Get())

	// we need to informer jobs, pods and all cloudshells, most of jobs and pods we don't care about.
	// so we need to select resources related to cloudshell to reduce the pressure of apiserver and
	// synchronically reduce the overhead of operator memory.
	labelSelector := labels.NewSelector()
	requirement, err := labels.NewRequirement(constants.CloudshellOwnerLabelKey, selection.Exists, []string{})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*requirement)

	mgr, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		Logger:                     klog.Background(),
		Scheme:                     gclient.NewSchema(),
		LeaderElection:             opts.LeaderElection.LeaderElect,
		LeaderElectionID:           opts.LeaderElection.ResourceName,
		LeaderElectionNamespace:    opts.LeaderElection.ResourceNamespace,
		LeaderElectionResourceLock: opts.LeaderElection.ResourceLock,
		HealthProbeBindAddress:     net.JoinHostPort(opts.BindAddress, strconv.Itoa(opts.SecurePort)),
		MetricsBindAddress:         opts.MetricsBindAddress,
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: gclient.NewSchema(),
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Pod{}: {
					Label: labelSelector,
				},
				&batchv1.Job{}: {
					Label: labelSelector,
				},
			},
		}),
	})
	if err != nil {
		klog.ErrorS(err, "failed to build controller manager")
		return err
	}

	if err = (&controllers.CloudShellReconciler{
		Client: mgr.GetClient(),
		Scheme: gclient.NewSchema(),
	}).SetupWithManager(mgr); err != nil {
		klog.ErrorS(err, "unable to create controller", "controller", "cloudshell")
		return err
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.ErrorS(err, "failed to add health check endpoint")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.ErrorS(err, "failed to add health check endpoint")
		return err
	}

	if err := mgr.Start(ctx); err != nil {
		klog.ErrorS(err, "controller manager exits unexpectedly")
		return err
	}

	return nil
}
