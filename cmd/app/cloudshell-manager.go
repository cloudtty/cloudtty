package app

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudtty/cloudtty/pkg/constants"
	"github.com/cloudtty/cloudtty/pkg/controllers"
	"github.com/cloudtty/cloudtty/pkg/generated/informers/externalversions"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"

	"github.com/cloudtty/cloudtty/cmd/app/config"
	"github.com/cloudtty/cloudtty/cmd/app/options"
	"github.com/cloudtty/cloudtty/pkg/utils/feature"
	"github.com/cloudtty/cloudtty/pkg/version"
	worerkpool "github.com/cloudtty/cloudtty/pkg/workerpool"
)

func init() {
	runtime.Must(logsv1.AddFeatureGates(feature.MutableFeatureGate))
}

// NewCloudShellManagerCommand creates a *cobra.Command object with default parameters
func NewCloudShellManagerCommand(ctx context.Context) *cobra.Command {
	opts, _ := options.NewOptions()

	cmd := &cobra.Command{
		Use:   "cloudshell-manager",
		Short: `Run this command in order to run cloudshell controller manager`,
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			// Activate logging as soon as possible, after that
			// show flags with the final logging configuration.
			if err := logsv1.ValidateAndApply(opts.Logs, feature.FeatureGate); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			cliflag.PrintFlags(cmd.Flags())

			config, err := opts.Config()
			if err != nil {
				return err
			}

			return Run(ctx, config)
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

	namedFlagSets := opts.Flags()
	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name(), logs.SkipLoggingConfigurationFlags())

	fs := cmd.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, namedFlagSets, cols)

	return cmd
}

func Run(ctx context.Context, config *config.Config) error {
	// To help debugging, immediately log version
	klog.Infof("cloudshell-controller-manager version: %s", version.Get())
	klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))

	if !config.LeaderElection.LeaderElect {
		return StartControllers(config, ctx.Done())
	}

	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// add an uniquifier so that two processes on the same host don't accidentally both become active
	id += "_" + string(uuid.NewUUID())

	rl, err := resourcelock.NewFromKubeconfig(
		config.LeaderElection.ResourceLock,
		config.LeaderElection.ResourceNamespace,
		config.LeaderElection.ResourceName,
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: config.EventRecorder,
		},
		config.Kubeconfig,
		config.LeaderElection.RenewDeadline.Duration,
	)
	if err != nil {
		return fmt.Errorf("failed to create resource lock: %w", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Name:          config.LeaderElection.ResourceName,
		Lock:          rl,
		LeaseDuration: config.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: config.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   config.LeaderElection.RetryPeriod.Duration,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(_ context.Context) {
				_ = StartControllers(config, ctx.Done())
			},
			OnStoppedLeading: func() {
				klog.InfoS("leaderelection lost")
			},
		},
	})

	return nil
}

func StartControllers(c *config.Config, stopCh <-chan struct{}) error {
	ownerExist, err := labels.NewRequirement(constants.WorkerOwnerLabelKey, selection.Exists, nil)
	if err != nil {
		return err
	}

	labelsSelector := labels.NewSelector().Add(*ownerExist)

	factory := informers.NewSharedInformerFactoryWithOptions(c.KubeClient, 0,
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelsSelector.String()
		}))

	podInformer := factory.Core().V1().Pods()
	pool := worerkpool.New(c.Client, c.CoreWorkerLimit, c.MaxWorkerLimit, c.ScaleInWorkerQueueDuration, podInformer)

	informerFactory := externalversions.NewSharedInformerFactory(c.CloudShellClient, 0)
	informer := informerFactory.Cloudshell().V1alpha1().CloudShells()
	controller := controllers.New(c.Client, c.KubeClient, c.Kubeconfig, pool, c.CloudShellImage, informer, podInformer)

	factory.Start(stopCh)
	informerFactory.Start(stopCh)

	go pool.Run(stopCh)
	go controller.Run(1, stopCh)

	<-stopCh
	return nil
}
