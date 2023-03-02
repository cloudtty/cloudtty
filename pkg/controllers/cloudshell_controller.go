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

package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/cloudtty/cloudtty/pkg/helper"
	"github.com/pkg/errors"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	"github.com/cloudtty/cloudtty/pkg/constants"
	"github.com/cloudtty/cloudtty/pkg/manifests"
	util "github.com/cloudtty/cloudtty/pkg/utils"
	"github.com/cloudtty/cloudtty/pkg/utils/feature"
)

const (
	// CloudshellControllerFinalizer is added to cloudshell to ensure Work as well as the
	// execution space (namespace) is deleted before itself is deleted.
	CloudshellControllerFinalizer = "cloudtty.io/cloudshell-controller"

	DefaultMaxConcurrentReconciles = 5
)

// CloudShellReconciler reconciles a CloudShell object
type CloudShellReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cloudshell.cloudtty.io,resources=cloudshells,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cloudshell.cloudtty.io,resources=cloudshells/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cloudshell.cloudtty.io,resources=cloudshells/finalizers,verbs=update
//+kubebuilder:rbac:groups="*",resources="*",verbs="*"
//+kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CloudShell object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (c *CloudShellReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("Reconciling cloudshell %s", req.NamespacedName.Name)

	cloudshell := &cloudshellv1alpha1.CloudShell{}
	if err := c.Get(ctx, req.NamespacedName, cloudshell); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}

	if !cloudshell.DeletionTimestamp.IsZero() {
		return c.removeCloudshell(ctx, cloudshell)
	}

	// as we not have webhook to init some nessary feild to cloudshell.
	// fill this default values to cloudshell after calling "syncCloudshell".
	if err := c.fillForCloudshell(ctx, cloudshell); err != nil {
		return ctrl.Result{Requeue: true}, nil
	}

	return c.syncCloudshell(ctx, cloudshell)
}

func (c *CloudShellReconciler) syncCloudshell(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) (ctrl.Result, error) {
	if IsCloudshellFinished(cloudshell) {
		if cloudshell.Spec.Cleanup {
			if err := c.Delete(ctx, cloudshell); err != nil {
				klog.ErrorS(err, "Failed to delete cloudshell", "cloudshell", klog.KObj(cloudshell))
				return ctrl.Result{Requeue: true}, nil
			}
		}
		klog.V(4).InfoS("cloudshell phase is to be finished", "cloudshell", klog.KObj(cloudshell))
		return ctrl.Result{}, nil
	}

	job, err := c.GetJobForCloudshell(ctx, cloudshell.Namespace, cloudshell)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, err
		}
		// TODO: when ttl timeout, whether create cloudshell again.
		if job, err = c.CreateCloudShellJob(ctx, cloudshell); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		if err := c.UpdateCloudshellStatus(ctx, cloudshell, cloudshellv1alpha1.PhaseCreatedJob); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	// if job had completed or failed, we think the job is done.
	// TODO: if job state is failed, should retry?
	if ok, jobStatus := IsJobFinished(job); ok {
		if err := c.UpdateCloudshellStatus(ctx, cloudshell, string(jobStatus)); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if job.Status.Active > 0 {
		if len(cloudshell.Status.AccessURL) == 0 {
			if err := c.CreateRouteRule(ctx, cloudshell); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}

		if ok, _ := c.isRunning(ctx, job); !ok {
			klog.V(4).InfoS("Cloudshell phase is not ready", "cloudshell", klog.KObj(cloudshell))
			return ctrl.Result{}, nil
		}
		if err := c.UpdateCloudshellStatus(ctx, cloudshell, cloudshellv1alpha1.PhaseReady); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}
	return ctrl.Result{}, nil
}

func (c *CloudShellReconciler) fillForCloudshell(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	if len(cloudshell.Spec.ExposeMode) == 0 {
		cloudshell.Spec.ExposeMode = cloudshellv1alpha1.ExposureServiceNodePort
	}

	// we consider that the ttl is too short is not meaningful.
	if cloudshell.Spec.Ttl < 300 {
		cloudshell.Spec.Ttl = 300
	}
	if len(cloudshell.Spec.CommandAction) == 0 {
		cloudshell.Spec.CommandAction = "bash"
	}
	return c.ensureFinalizer(cloudshell)
}

func (c *CloudShellReconciler) removeFinalizer(cloudshell *cloudshellv1alpha1.CloudShell) error {
	if !ctrlutil.ContainsFinalizer(cloudshell, CloudshellControllerFinalizer) {
		return nil
	}

	ctrlutil.RemoveFinalizer(cloudshell, CloudshellControllerFinalizer)
	return c.Client.Update(context.TODO(), cloudshell)
}

func (c *CloudShellReconciler) ensureFinalizer(cloudshell *cloudshellv1alpha1.CloudShell) error {
	if ctrlutil.ContainsFinalizer(cloudshell, CloudshellControllerFinalizer) {
		return nil
	}

	ctrlutil.AddFinalizer(cloudshell, CloudshellControllerFinalizer)
	return c.Client.Update(context.TODO(), cloudshell)
}

// CreateCloudShellJob clould create a job for cloudshell, the job will running a cloudtty server in the pod.
// the job template set default images registry "ghcr.io" and default command, no modification is supported currently,
// the configmap must be existed in the cluster.
func (c *CloudShellReconciler) CreateCloudShellJob(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) (*batchv1.Job, error) {
	// if configmap is blank, use the Incluster rest config to generate kubeconfig and restore a configmap.
	// the kubeconfig only work on current cluster.
	if feature.FeatureGate.Enabled(AllowSecretStoreKubeconfig) && cloudshell.Spec.SecretRef == nil {
		kubeConfigByte, err := GenerateKubeconfigInCluster()
		if err != nil {
			return nil, err
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("cloudshell-%s-", cloudshell.Name),
				Namespace:    cloudshell.Namespace,
			},
			Data: map[string][]byte{"config": kubeConfigByte},
		}

		if err := ctrlutil.SetControllerReference(cloudshell, secret, c.Scheme); err != nil {
			klog.ErrorS(err, "Failed to set owner reference for configmap", "cloudshell", klog.KObj(cloudshell))
			return nil, err
		}
		if err := c.Client.Create(ctx, secret); err != nil {
			return nil, err
		}
		cloudshell.Spec.SecretRef = &cloudshellv1alpha1.LocalSecretReference{
			Name: secret.Name,
		}
	}

	jobTmpl := manifests.JobTmplV1
	if template, err := util.LoadYamlTemplate(constants.JobTemplatePath); err != nil {
		klog.V(2).InfoS("failed to load job template from /etc/cloudtty", "err", err)
	} else {
		jobTmpl = template
	}

	jobBytes, err := util.ParseTemplate(jobTmpl, helper.NewPodTemplateValue(cloudshell))
	if err != nil {
		return nil, errors.Wrap(err, "failed create cloudshell job")
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(jobBytes, nil, nil)
	if err != nil {
		klog.ErrorS(err, "failed to decode job manifest")
		return nil, err
	}
	job := obj.(*batchv1.Job)

	// set CloudshellOwnerLabelKey label for job and its pods. we can informer the exact resource.
	job.SetLabels(map[string]string{constants.CloudshellOwnerLabelKey: cloudshell.Name})
	job.Spec.Template.SetLabels(map[string]string{constants.CloudshellOwnerLabelKey: cloudshell.Name})

	if len(cloudshell.Spec.Image) > 0 {
		for i, container := range job.Spec.Template.Spec.Containers {
			if container.Name == constants.DefauletWebttyContainerName {
				job.Spec.Template.Spec.Containers[i].Image = cloudshell.Spec.Image
			}
		}
	}
	if len(cloudshell.Spec.Env) > 0 {
		for i, container := range job.Spec.Template.Spec.Containers {
			if container.Name == constants.DefauletWebttyContainerName {
				job.Spec.Template.Spec.Containers[i].Env =
					append(job.Spec.Template.Spec.Containers[i].Env, cloudshell.Spec.Env...)
			}
		}
	}

	if err := ctrlutil.SetControllerReference(cloudshell, job, c.Scheme); err != nil {
		klog.ErrorS(err, "failed to set owner reference for job", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}

	if err := c.Create(ctx, job); err != nil {
		klog.ErrorS(err, "failed to create job", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}
	return job, nil
}

// CreateRouteRule create a service resource in the same namespace of cloudshell no matter what expose model.
// if the expose model is ingress or virtualService, it will create additional resources, e.g: ingress or virtualService.
// and the accressUrl will be update.
func (c *CloudShellReconciler) CreateRouteRule(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	service, err := c.GetServiceForCloudshell(ctx, cloudshell.Namespace, cloudshell)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if service, err = c.CreateCloudShellService(ctx, cloudshell); err != nil {
			return err
		}
	}

	var accessURL string
	switch cloudshell.Spec.ExposeMode {
	case cloudshellv1alpha1.ExposureServiceClusterIP:
		accessURL = fmt.Sprintf("%s:%d", service.Spec.ClusterIP, constants.DefaultServicePort)
	case "", cloudshellv1alpha1.ExposureServiceNodePort:
		// Default(No explicit `ExposeMode` specified in CR) mode is Nodeport
		host, err := c.GetMasterNodeIP(ctx)
		if err != nil {
			klog.InfoS("Unable to get master node IP addr", "cloudshell", klog.KObj(cloudshell), "err", err)
		}

		var nodePort int32
		// TODO: nodePort may be blank due to delay filled by k8s, should `ctrl.Result{RequeueAfter: 5}, nil`
		if service.Spec.Type == corev1.ServiceTypeNodePort {
			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					nodePort = port.NodePort
					break
				}
			}
		}
		accessURL = fmt.Sprintf("%s:%d", host, nodePort)
	case cloudshellv1alpha1.ExposureIngress:
		if err := c.CreateIngressForCloudshell(ctx, service, cloudshell); err != nil {
			klog.ErrorS(err, "failed create ingress for cloudshell", "cloudshell", klog.KObj(cloudshell))
			return err
		}
		accessURL = SetRouteRulePath(cloudshell)
	case cloudshellv1alpha1.ExposureVirtualService:
		if err := c.CreateVitualServiceForCloudshell(ctx, service, cloudshell); err != nil {
			klog.ErrorS(err, "failed create virtualservice for cloudshell", "cloudshell", klog.KObj(cloudshell))
			return err
		}
		accessURL = SetRouteRulePath(cloudshell)
	}

	cloudshell.Status.AccessURL = accessURL
	return c.UpdateCloudshellStatus(ctx, cloudshell, cloudshellv1alpha1.PhaseCreatedRoute)
}

// GetJobForCloudshell to find job of cloudshell according to labels ""cloudshell.cloudtty.io/owner-name"".
func (c *CloudShellReconciler) GetJobForCloudshell(ctx context.Context, namespace string, cloudshell *cloudshellv1alpha1.CloudShell) (*batchv1.Job, error) {
	var jobs batchv1.JobList
	if err := c.List(ctx, &jobs, client.InNamespace(namespace), client.MatchingLabels{constants.CloudshellOwnerLabelKey: cloudshell.Name}); err != nil {
		return nil, err
	}
	if len(jobs.Items) > 1 {
		klog.InfoS("found multiple cloudshell jobs", "cloudshell", klog.KObj(cloudshell))
		return nil, errors.New("found multiple cloudshell jobs")
	}
	if len(jobs.Items) == 0 {
		return nil, apierrors.NewNotFound(batchv1.Resource("jobs"), fmt.Sprintf("cloudshell-%s", cloudshell.Name))
	}

	return &jobs.Items[0], nil
}

// GetMasterNodeIP could find the one master node IP.
func (c *CloudShellReconciler) GetMasterNodeIP(ctx context.Context) (string, error) {
	// the label "node-role.kubernetes.io/master" be removed in k8s 1.24, and replace with
	// "node-role.kubernetes.io/cotrol-plane".
	nodes := &corev1.NodeList{}
	if err := c.List(ctx, nodes, client.MatchingLabels{"node-role.kubernetes.io/master": ""}); err != nil {
		return "", err
	}
	if len(nodes.Items) == 0 {
		if err := c.List(ctx, nodes, client.MatchingLabels{"node-role.kubernetes.io/control-plane": ""}); err != nil || len(nodes.Items) == 0 {
			return "", err
		}
	}

	for _, addr := range nodes.Items[0].Status.Addresses {
		// Using External IP as first priority
		if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", nil
}

// GetServiceForCloudshell to find service of cloudshell according to labels "cloudshell.cloudtty.io/owner-name".
func (c *CloudShellReconciler) GetServiceForCloudshell(ctx context.Context, namespace string, cloudshell *cloudshellv1alpha1.CloudShell) (*corev1.Service, error) {
	var services corev1.ServiceList
	if err := c.List(ctx, &services, client.InNamespace(namespace), client.MatchingLabels{constants.CloudshellOwnerLabelKey: cloudshell.Name}); err != nil {
		return nil, err
	}
	if len(services.Items) > 1 {
		klog.InfoS("found multiple cloudshell service", "cloudshell", klog.KObj(cloudshell))
		return nil, errors.New("found multiple cloudshell services")
	}
	if len(services.Items) == 0 {
		return nil, apierrors.NewNotFound(corev1.Resource("services"), fmt.Sprintf("cloudshell-%s", cloudshell.Name))
	}
	return &services.Items[0], nil
}

// CreateCloudShellService Create service resource for cloudshell, the service type is either ClusterIP, NodePort,
// Ingress or virtualService. if the expose model is ingress or virtualService. it will create clusterIP type service.
func (c *CloudShellReconciler) CreateCloudShellService(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) (*corev1.Service, error) {
	serviceType := cloudshell.Spec.ExposeMode
	if len(serviceType) == 0 {
		serviceType = cloudshellv1alpha1.ExposureServiceNodePort
	}
	// if ExposeMode is ingress or vituralService, the svc type should be ClusterIP.
	if serviceType == cloudshellv1alpha1.ExposureIngress ||
		serviceType == cloudshellv1alpha1.ExposureVirtualService {
		serviceType = cloudshellv1alpha1.ExposureServiceClusterIP
	}

	serviceBytes, err := util.ParseTemplate(manifests.ServiceTmplV1, helper.NewServiceTemplateValue(cloudshell, serviceType))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse cloudshell service manifest")
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(serviceBytes, nil, nil)
	if err != nil {
		klog.ErrorS(err, "failed to decode service manifest", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}
	svc := obj.(*corev1.Service)
	svc.SetLabels(map[string]string{constants.CloudshellOwnerLabelKey: cloudshell.Name})

	// set reference for service, once the cloudshell is deleted, the service is alse deleted.
	if err := ctrlutil.SetControllerReference(cloudshell, svc, c.Scheme); err != nil {
		return nil, err
	}

	if err := c.Create(ctx, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

// CreateIngressForCloudshell create ingress for cloudshell, if there isn't an ingress controller server
// in the cluster, the ingress is still not working. before create ingress, there's must a service
// as the ingress backend service. all of services should be loaded in an ingress "cloudshell-ingress".
func (c *CloudShellReconciler) CreateIngressForCloudshell(ctx context.Context, service *corev1.Service, cloudshell *cloudshellv1alpha1.CloudShell) error {
	ingress := &networkingv1.Ingress{}
	objectKey := IngressNamespacedName(cloudshell)
	err := c.Get(ctx, objectKey, ingress)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// if there is not ingress in the cluster, create the base ingress.
	if ingress != nil && apierrors.IsNotFound(err) {
		var ingressClassName string
		if cloudshell.Spec.IngressConfig != nil && len(cloudshell.Spec.IngressConfig.IngressClassName) > 0 {
			ingressClassName = cloudshell.Spec.IngressConfig.IngressClassName
		}

		// set default path prefix.
		rulePath := SetRouteRulePath(cloudshell)
		ingressTemplateValue := helper.NewIngressTemplateValue(objectKey, ingressClassName, service.Name, rulePath)
		ingressBytes, err := util.ParseTemplate(manifests.IngressTmplV1, ingressTemplateValue)

		if err != nil {
			return errors.Wrap(err, "failed to parse cloudshell ingress manifest")
		}

		decoder := scheme.Codecs.UniversalDeserializer()
		obj, _, err := decoder.Decode(ingressBytes, nil, nil)
		if err != nil {
			klog.ErrorS(err, "failed to decode ingress manifest", "cloudshell", klog.KObj(cloudshell))
			return err
		}
		ingress = obj.(*networkingv1.Ingress)
		ingress.SetLabels(map[string]string{constants.CloudshellOwnerLabelKey: cloudshell.Name})

		return c.Create(ctx, ingress)
	}

	// there is an ingress in the cluster, add a rule to the ingress.
	IngressRule := ingress.Spec.Rules[0].IngressRuleValue.HTTP
	newPathRule := IngressRule.Paths[0].DeepCopy()

	newPathRule.Backend.Service.Name = service.Name
	newPathRule.Path = SetRouteRulePath(cloudshell)
	IngressRule.Paths = append(IngressRule.Paths, *newPathRule)
	return c.Update(ctx, ingress)
}

// CreateVitualServiceForCloudshell create a virtualService resource in the cluster. if there
// is no istio server be deployed in the cluster. will not create the resource.
func (c *CloudShellReconciler) CreateVitualServiceForCloudshell(ctx context.Context, service *corev1.Service, cloudshell *cloudshellv1alpha1.CloudShell) error {
	config := cloudshell.Spec.VirtualServiceConfig
	if config == nil {
		return errors.New("unable create virtualservice, missing configuration options")
	}

	// TODO: check the crd in the cluster.

	objectKey := VsNamespacedName(cloudshell)
	virtualService := &istionetworkingv1beta1.VirtualService{}

	err := c.Get(ctx, objectKey, virtualService)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// if there is not virtualService in the cluster, create the base virtualService.
	if virtualService != nil && apierrors.IsNotFound(err) {
		rulePath := SetRouteRulePath(cloudshell)
		virtualServiceTemplateValue := helper.NewVirtualServiceTemplateValue(objectKey, config, service.Name, rulePath)
		vitualServiceBytes, err := util.ParseTemplate(manifests.VirtualServiceV1Beta1, virtualServiceTemplateValue)
		if err != nil {
			return errors.Wrapf(err, "failed to parse cloudshell [%s] virtualservice", cloudshell.Name)
		}

		scheme := runtime.NewScheme()
		istionetworkingv1beta1.AddToScheme(scheme)
		decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()
		obj, _, err := decoder.Decode(vitualServiceBytes, nil, nil)
		if err != nil {
			klog.ErrorS(err, "failed to decode virtualservice manifest", "cloudshell", klog.KObj(cloudshell))
			return err
		}
		virtualService = obj.(*istionetworkingv1beta1.VirtualService)
		virtualService.SetLabels(map[string]string{constants.CloudshellOwnerLabelKey: cloudshell.Name})

		return c.Create(ctx, virtualService)
	}

	// there is a virtualService in the cluster, add a destination to it.
	newHttpRoute := virtualService.Spec.Http[0].DeepCopy()
	newHttpRoute.Match[0].Uri.MatchType = &networkingv1beta1.StringMatch_Prefix{Prefix: SetRouteRulePath(cloudshell)}
	newHttpRoute.Route[0].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, objectKey.Namespace)

	virtualService.Spec.Http = append(virtualService.Spec.Http, newHttpRoute)
	return c.Update(ctx, virtualService)
}

// UpdateCloudshellStatus update the clodushell status.
func (c *CloudShellReconciler) UpdateCloudshellStatus(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell, phase string) error {
	firstTry := true
	cloudshell.Status.Phase = phase
	status := cloudshell.Status
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if !firstTry {
			if getErr := c.Get(ctx, types.NamespacedName{Name: cloudshell.Name, Namespace: cloudshell.Namespace}, cloudshell); err != nil {
				return getErr
			}
		}
		cloudshell.Status = status
		cc := cloudshell.DeepCopy()

		err = c.Status().Update(ctx, cc)
		firstTry = false
		return
	})
}

func (c *CloudShellReconciler) removeCloudshell(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) (ctrl.Result, error) {
	if err := c.removeCloudshellRoute(ctx, cloudshell); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	if err := c.removeFinalizer(cloudshell); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	klog.V(4).InfoS("delete cloudshell", "cloudshell", klog.KObj(cloudshell))
	return ctrl.Result{}, nil
}

// removeCloudshell remove the cloudshell, at the same time, update addition resource.
// i.g: ingress or vitualService. if all of cloudshells was removed, it will delete the
// ingress or vitualService.
func (c *CloudShellReconciler) removeCloudshellRoute(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	// delete route rule.
	switch cloudshell.Spec.ExposeMode {
	case "", cloudshellv1alpha1.ExposureServiceClusterIP, cloudshellv1alpha1.ExposureServiceNodePort:
		// TODO: whether to delete ownReference resource.
	case cloudshellv1alpha1.ExposureIngress:
		ingress := &networkingv1.Ingress{}
		if err := c.Get(ctx, IngressNamespacedName(cloudshell), ingress); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		ingressRule := ingress.Spec.Rules[0].IngressRuleValue.HTTP

		// remove rule from ingress. if the length of ingressRule is zero, delete it directly.
		for i := 0; i < len(ingressRule.Paths); i++ {
			if ingressRule.Paths[i].Path == cloudshell.Status.AccessURL {
				ingressRule.Paths = append(ingressRule.Paths[:i], ingressRule.Paths[i+1:]...)
				break
			}
		}

		if len(ingressRule.Paths) == 0 {
			return c.Delete(ctx, ingress)
		}
		return c.Update(ctx, ingress)
	case cloudshellv1alpha1.ExposureVirtualService:
		virtualService := &istionetworkingv1beta1.VirtualService{}
		if err := c.Get(ctx, VsNamespacedName(cloudshell), virtualService); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		// remove rule from virtualService. if the length of virtualService is zero, delete it directly.
		httpRoute := virtualService.Spec.Http
		for i := 0; i < len(httpRoute); i++ {
			match := httpRoute[i].Match
			if len(match) > 0 && match[0].Uri != nil {
				if prefix, ok := match[0].Uri.MatchType.(*networkingv1beta1.StringMatch_Prefix); ok &&
					prefix.Prefix == cloudshell.Status.AccessURL {

					httpRoute = append(httpRoute[:i], httpRoute[i+1:]...)
					break
				}
			}
		}

		if len(httpRoute) == 0 {
			return c.Delete(ctx, virtualService)
		}

		virtualService.Spec.Http = httpRoute
		return c.Update(ctx, virtualService)
	}
	return nil
}

// SetRouteRulePath return access url according to cloudshell.
func SetRouteRulePath(cloudshell *cloudshellv1alpha1.CloudShell) string {
	var pathPrefix string
	if len(cloudshell.Spec.PathPrefix) != 0 {
		pathPrefix = cloudshell.Spec.PathPrefix
	}

	if strings.HasSuffix(pathPrefix, "/") {
		pathPrefix = pathPrefix[:len(pathPrefix)-1] + constants.DefaultPathPrefix
	} else {
		pathPrefix += constants.DefaultPathPrefix
	}

	if len(cloudshell.Spec.PathSuffix) > 0 {
		return fmt.Sprintf("%s/%s/%s", pathPrefix, cloudshell.Name, cloudshell.Spec.PathSuffix)
	}
	return fmt.Sprintf("%s/%s", pathPrefix, cloudshell.Name)
}

// IngressNamespacedName return a namespacedName accroding to ingressConfig.
func IngressNamespacedName(cloudshell *cloudshellv1alpha1.CloudShell) types.NamespacedName {
	// set custom name and namespace to ingress.
	config := cloudshell.Spec.IngressConfig
	ingressName := constants.DefaultIngressName
	if config != nil && len(config.IngressName) > 0 {
		ingressName = config.IngressName
	}

	namespace := cloudshell.Namespace
	if config != nil && len(config.Namespace) > 0 {
		namespace = config.Namespace
	}

	return types.NamespacedName{Name: ingressName, Namespace: namespace}
}

// VsNamespacedName return a namespacedName accroding to virtaulServiceConfig.
func VsNamespacedName(cloudshell *cloudshellv1alpha1.CloudShell) types.NamespacedName {
	// set custom name and namespace to ingress.
	config := cloudshell.Spec.VirtualServiceConfig
	vsName := constants.DefaultVirtualServiceName
	if config != nil && len(config.VirtualServiceName) > 0 {
		vsName = config.VirtualServiceName
	}

	namespace := cloudshell.Namespace
	if config != nil && len(config.Namespace) > 0 {
		namespace = config.Namespace
	}

	return types.NamespacedName{Name: vsName, Namespace: namespace}
}

// isRunning check pod of job whether running, only one of the pods is running,
// and be considered the cloudtty server is working.

// TODO: The field `job.status.Ready` is alpha phase. we can depend on the field if it's to be beta.
func (c *CloudShellReconciler) isRunning(ctx context.Context, job *batchv1.Job) (bool, error) {
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(job.Namespace), client.MatchingLabels{"job-name": job.Name}); err != nil {
		return false, err
	}
	for _, p := range pods.Items {
		if p.Status.Phase != corev1.PodRunning || p.DeletionTimestamp != nil {
			continue
		}
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
	}

	return false, errors.Errorf("no pod of job %s is running", job.Name)
}

// findObjectsForPod define the relation between pod and cloudshell. we can find a
// cloudshell by the label of pod.
func (c *CloudShellReconciler) findObjectsForPod(pod client.Object) []reconcile.Request {
	var cloudshellName string
	for label, value := range pod.GetLabels() {
		if label == constants.CloudshellOwnerLabelKey {
			cloudshellName = value
			break
		}
	}
	if len(cloudshellName) == 0 {
		return []reconcile.Request{}
	}
	attachedCloudshell := &cloudshellv1alpha1.CloudShell{}
	err := c.Get(context.TODO(), types.NamespacedName{Namespace: pod.GetNamespace(), Name: cloudshellName}, attachedCloudshell)
	if err != nil {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{
			Name:      attachedCloudshell.GetName(),
			Namespace: attachedCloudshell.GetNamespace(),
		}},
	}
}

// GenerateKubeconfigByInCluster load serviceaccount info under
// "/var/run/secrets/kubernetes.io/serviceaccount" and generate kubeconfig str.
func GenerateKubeconfigInCluster() ([]byte, error) {
	const (
		tokenFile  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
		rootCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	)

	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, errors.New("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}

	token, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	rootCA, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return nil, err
	}

	kubeConfigTemplateValue := helper.NewKubeConfigTemplateValue(host, port, string(token), rootCA)
	return util.ParseTemplate(manifests.KubeconfigTmplV1, kubeConfigTemplateValue)
}

// SetupWithManager sets up the controller with the Manager.
func (c *CloudShellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cloudshellv1alpha1.CloudShell{},
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: OnCloudshellUpdate,
			})).
		Owns(&batchv1.Job{},
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc:  OnJobUpdate,
				DeleteFunc:  func(event.DeleteEvent) bool { return false },
				CreateFunc:  func(event.CreateEvent) bool { return false },
				GenericFunc: func(event.GenericEvent) bool { return false },

				// TODO: the job be deleted during the ttl of cloudshell,
				// should create job again？
			})).
		Watches(&source.Kind{Type: &corev1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(c.findObjectsForPod),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc:  OnPodUpdate,
				DeleteFunc:  func(event.DeleteEvent) bool { return false },
				CreateFunc:  func(event.CreateEvent) bool { return false },
				GenericFunc: func(event.GenericEvent) bool { return false },
			})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: DefaultMaxConcurrentReconciles,
		}).
		Complete(c)
}

// OnCloudshellUpdate difine update event to cloudshell, in following case, we need take
// cloudshell requeue:
// 1. delete kmd cr.
// 2. if the kmd spec is changed, we need to reconcile again.
func OnCloudshellUpdate(updateEvent event.UpdateEvent) bool {
	newObj := updateEvent.ObjectNew.(*cloudshellv1alpha1.CloudShell)
	oldObj := updateEvent.ObjectOld.(*cloudshellv1alpha1.CloudShell)

	return newObj.DeletionTimestamp != nil || !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
}

// OnJobUpdate difine update event to job, in following case, we need take
// cloudshell requeue:
// 1. the active of job from 0 to 1, the number of pending and running pods.
// 2. the state of job is to be completed.
// 3. the state of job is to be failed.
func OnJobUpdate(updateEvent event.UpdateEvent) bool {
	newObj := updateEvent.ObjectNew.(*batchv1.Job)
	oldObj := updateEvent.ObjectOld.(*batchv1.Job)
	if newObj.Status.Active != oldObj.Status.Active {
		return true
	}
	if finished, _ := IsJobFinished(newObj); finished {
		return true
	}
	return false
}

// OnJobUpdate difine update event to pod, in following case, we need take
// cloudshell requeue:
// 1. the pod status is running.
func OnPodUpdate(updateEvent event.UpdateEvent) bool {
	newObj := updateEvent.ObjectNew.(*corev1.Pod)
	oldObj := updateEvent.ObjectOld.(*corev1.Pod)

	if newObj.DeletionTimestamp != nil {
		return false
	}
	if !reflect.DeepEqual(newObj.Status.Conditions, oldObj.Status.Conditions) {
		for _, c := range newObj.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return true
			}
		}
	}

	return false
}
