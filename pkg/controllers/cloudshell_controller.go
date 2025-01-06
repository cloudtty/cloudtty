package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
	"github.com/cloudtty/cloudtty/pkg/constants"
	cloudshellinformers "github.com/cloudtty/cloudtty/pkg/generated/informers/externalversions/cloudshell/v1alpha1"
	cloudshellisters "github.com/cloudtty/cloudtty/pkg/generated/listers/cloudshell/v1alpha1"
	"github.com/cloudtty/cloudtty/pkg/manifests"
	util "github.com/cloudtty/cloudtty/pkg/utils"
	"github.com/cloudtty/cloudtty/pkg/utils/gclient"
	worerkpool "github.com/cloudtty/cloudtty/pkg/workerpool"
)

const (
	// CloudshellControllerFinalizer is added to cloudshell to ensure Work as well as the
	// execution space (namespace) is deleted before itself is deleted.
	CloudshellControllerFinalizer = "cloudtty.io/cloudshell-controller"

	// DefaultCloudShellBackOff is the default backoff period. Exported for tests.
	DefaultCloudShellBackOff = 2 * time.Second

	// MaxCloudShellBackOff is the max backoff period. Exported for tests.
	MaxCloudShellBackOff = 5 * time.Second

	startupScriptPath = "/usr/lib/ttyd/startup.sh"
	resetScriptPath   = "/usr/lib/ttyd/reset.sh"

	CloudshellConfigMapLabel = "cloudtty.io/cloudshell-config"
)

// Controller reconciles a CloudShell object
type Controller struct {
	client.Client
	kubeClient kubernetes.Interface
	config     *rest.Config
	Scheme     *runtime.Scheme
	workerPool *worerkpool.WorkerPool

	queue              workqueue.RateLimitingInterface
	cloudshellInformer cache.SharedIndexInformer
	lister             cloudshellisters.CloudShellLister
	podInformer        cache.SharedIndexInformer
	podLister          listerscorev1.PodLister

	ttydServiceBufferSize string

	cloudshellImage string
}

func New(client client.Client, kubeClient kubernetes.Interface, config *rest.Config, wp *worerkpool.WorkerPool, cloudshellImage string,
	cloudshellInformer cloudshellinformers.CloudShellInformer, podInformer informercorev1.PodInformer,
) *Controller {
	controller := &Controller{
		Client:     client,
		kubeClient: kubeClient,
		config:     config,
		Scheme:     gclient.NewSchema(),
		workerPool: wp,
		queue: workqueue.NewRateLimitingQueue(
			workqueue.NewItemExponentialFailureRateLimiter(DefaultCloudShellBackOff, MaxCloudShellBackOff),
		),

		cloudshellInformer: cloudshellInformer.Informer(),
		lister:             cloudshellInformer.Lister(),
		podInformer:        podInformer.Informer(),
		podLister:          podInformer.Lister(),
		cloudshellImage:    cloudshellImage,
	}

	_, err := cloudshellInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				controller.enqueue(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				older := oldObj.(*cloudshellv1alpha1.CloudShell)
				newer := newObj.(*cloudshellv1alpha1.CloudShell)
				if !reflect.DeepEqual(older.Spec, newer.Spec) || !newer.DeletionTimestamp.IsZero() {
					controller.enqueue(newObj)
				}
			},
			DeleteFunc: func(obj interface{}) {
				controller.enqueue(obj)
			},
		},
	)
	if err != nil {
		klog.ErrorS(err, "error when adding event handler to informer")
	}

	return controller
}

func (c *Controller) enqueue(obj interface{}) {
	cloudshell := obj.(*cloudshellv1alpha1.CloudShell)
	key, _ := cache.MetaNamespaceKeyFunc(cloudshell)
	c.queue.Add(key)
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	// main reconcile logic
	requeueAfter, err := c.syncHandler(context.TODO(), key.(string))
	switch {
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("sync %q failed with %v", key, err))
		c.queue.AddRateLimited(key)
	case requeueAfter != nil:
		c.queue.Forget(key)
		c.queue.AddAfter(key, *requeueAfter)
	}

	return true
}

func (c *Controller) worker(stopCh <-chan struct{}) {
	for c.processNextItem() {
		select {
		case <-stopCh:
			return
		default:
		}
	}
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Start CloudShell Controller")
	defer klog.InfoS("Shutting down CloudShell Controller")

	if !cache.WaitForCacheSync(stopCh, c.cloudshellInformer.HasSynced) {
		klog.Errorf("cloudshell manager: wait for cloushell informer factory failed")
	}
	if !cache.WaitForCacheSync(stopCh, c.podInformer.HasSynced) {
		klog.Errorf("cloudshell manager: wait for pod informer factory failed")
	}
	currentNS := util.GetCurrentNSOrDefault()
	configs, err := c.kubeClient.CoreV1().ConfigMaps(currentNS).List(context.TODO(), metav1.ListOptions{LabelSelector: CloudshellConfigMapLabel})
	if err != nil {
		klog.ErrorS(err, "failed to list CloudShell ConfigMap")
	} else {
		if len(configs.Items) > 0 {
			c.ttydServiceBufferSize = configs.Items[0].Data["TTYD_SERVER_BUFFER_SIZE"]
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.worker(stopCh)
		}()
	}
	<-stopCh

	c.queue.ShutDown()
	wg.Wait()
}

func (c *Controller) syncHandler(ctx context.Context, key string) (*time.Duration, error) {
	klog.V(4).Infof("Reconciling cloudshell %s", key)
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing %q (%v)", key, time.Since(startTime))
	}()

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	cloudShell, err := c.lister.CloudShells(ns).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).InfoS("Skip syncing, cloudshell not found", "cloudshell", key)
			return nil, nil
		}

		return nil, err
	}

	if !cloudShell.DeletionTimestamp.IsZero() {
		return nil, c.removeCloudshell(ctx, cloudShell)
	}

	// as we not have webhook to init some necessary feild to cloudshell.
	// fill this default values to cloudshell after calling "syncCloudShell".
	if err := c.ensureCloudShell(ctx, cloudShell); err != nil {
		return nil, err
	}

	return c.syncCloudShell(ctx, cloudShell)
}

func (c *Controller) syncCloudShell(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) (*time.Duration, error) {
	t := nextRequeueTimeDuration(cloudshell)

	// remove cloudshell while ttl is timeout.
	if t != nil && *t < 0 {
		if err := c.removeCloudshell(ctx, cloudshell); err != nil {
			return nil, err
		}
		if cloudshell.Spec.Cleanup {
			return nil, c.Delete(ctx, cloudshell)
		}

		return nil, c.UpdateCloudshellStatus(ctx, cloudshell, cloudshellv1alpha1.PhaseCompleted)
	}

	worker, err := c.GetBindingWorkerFor(cloudshell)
	if err != nil {
		return nil, fmt.Errorf("failed to get binding worker for cloudshell, err: %v", err)
	}

	// TODO: when cloudshell image be changed, the image of binding worker is different with the spce.image

	if worker == nil {
		req := &worerkpool.Request{
			Cloudshell:      cloudshell.GetName(),
			Namespace:       cloudshell.GetNamespace(),
			Image:           cloudshell.Spec.Image,
			CloudShellQueue: c.queue,
		}

		worker, err = c.workerPool.Borrow(req)
		if err != nil {
			if err == worerkpool.ErrNotWorker {
				klog.InfoS("wait worker poll to create worker", "cloudshell", klog.KObj(cloudshell))
				return nil, nil
			}

			klog.ErrorS(err, "failed to borrow worker for pool", "cloudshell", klog.KObj(cloudshell))
			return nil, err
		}

		AddLabel(cloudshell, constants.CloudshellPodLabelKey, worker.Name)
		if err := c.Update(context.TODO(), cloudshell); err != nil {
			return nil, err
		}
	}

	url, err := c.CreateRouteRule(ctx, cloudshell, worker)
	if err != nil {
		klog.ErrorS(err, "failed to create route rule for cloudshell", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}

	if err = c.StartupWorkerFor(ctx, cloudshell); err != nil {
		klog.ErrorS(err, "failed to start pod for cloudshell", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}

	// url add ttl param, if ttl was set
	if cloudshell.Spec.TTLSecondsAfterStarted != nil {
		accessURLFmtStr := "%s?ttl=%d"
		url = fmt.Sprintf(accessURLFmtStr, url, *cloudshell.Spec.TTLSecondsAfterStarted)
	}
	cloudshell.Status.AccessURL = url

	if cloudshell.Status.LastScheduleTime == nil {
		now := metav1.Now()
		cloudshell.Status.LastScheduleTime = &now
	}

	return t, c.UpdateCloudshellStatus(ctx, cloudshell, cloudshellv1alpha1.PhaseReady)
}

func nextRequeueTimeDuration(cloudshell *cloudshellv1alpha1.CloudShell) *time.Duration {
	if cloudshell.Spec.TTLSecondsAfterStarted != nil {
		startTime := metav1.Now()
		if cloudshell.Status.LastScheduleTime != nil {
			startTime = *cloudshell.Status.LastScheduleTime
		}

		ttl := time.Second * time.Duration(*cloudshell.Spec.TTLSecondsAfterStarted)

		duration := time.Until(startTime.Add(ttl))
		return &duration
	}

	return nil
}

// StartupWorkerFor copy kubeConfig and start ttyd
func (c *Controller) StartupWorkerFor(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	// if secretRef is empty, use the Incluster rest config to generate kubeconfig and restore a secret.
	// the kubeconfig only work on current cluster.
	var kubeConfigByte []byte
	if cloudshell.Spec.SecretRef == nil {
		kubeConfigByte, err := GenerateKubeconfigInCluster()
		if err != nil {
			return err
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
			return err
		}
		if err := c.Client.Create(ctx, secret); err != nil {
			return err
		}
		cloudshell.Spec.SecretRef = &cloudshellv1alpha1.LocalSecretReference{
			Name: secret.Name,
		}
	} else {
		secret := &corev1.Secret{}
		err := c.Client.Get(ctx, client.ObjectKey{Namespace: cloudshell.Namespace, Name: cloudshell.Spec.SecretRef.Name}, secret)
		if err != nil {
			return err
		}

		kubeConfigByte = secret.Data["config"]
	}

	return c.StartupWorker(ctx, cloudshell, kubeConfigByte)
}

func (c *Controller) StartupWorker(_ context.Context, cloudshell *cloudshellv1alpha1.CloudShell, kubeConfigByte []byte) error {
	// TODO: Some extra logic in order to upload and download files.
	var podName, namespace, container, serverBufferSize, ps1 string
	serverBufferSize = c.ttydServiceBufferSize
	for _, env := range cloudshell.Spec.Env {
		switch env.Name {
		case "POD_NAME":
			podName = env.Value
		case "POD_NAMESPACE":
			namespace = env.Value
		case "CONTAINER":
			container = env.Value
		case "TTYD_SERVER_BUFFER_SIZE":
			serverBufferSize = env.Value
		case "PS1":
			ps1 = env.Value
		}
	}
	klog.InfoS("Cloudshell config", "cloudshell.name", cloudshell.Name, "serverBufferSize", serverBufferSize)
	// start ttyd, ttyd args passed as shell parameter
	// case: ttydCommand := []string{"/usr/lib/ttyd/startup.sh", "${KUBECONFIG}" "${ONCE}", "${URLARG}", "${COMMAND}"}
	ttydCommand := []string{
		startupScriptPath,
		string(kubeConfigByte), fmt.Sprint(cloudshell.Spec.Once), fmt.Sprint(cloudshell.Spec.UrlArg),
		cloudshell.Spec.CommandAction, podName, namespace, container, ps1,
	}
	if serverBufferSize != "" {
		ttydCommand = append(ttydCommand, serverBufferSize)
	}
	return execCommand(cloudshell, ttydCommand, c.config)
}

func (c *Controller) removeFinalizer(cloudshell *cloudshellv1alpha1.CloudShell) error {
	if ctrlutil.RemoveFinalizer(cloudshell, CloudshellControllerFinalizer) {
		return c.Client.Update(context.TODO(), cloudshell)
	}

	return nil
}

func (c *Controller) ensureCloudShell(_ context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	updated := ctrlutil.AddFinalizer(cloudshell, CloudshellControllerFinalizer)

	older := cloudshell.DeepCopy()

	if len(cloudshell.Spec.Image) == 0 {
		cloudshell.Spec.Image = c.cloudshellImage
	}
	gclient.NewSchema().Default(cloudshell)

	if updated || !reflect.DeepEqual(cloudshell.Spec, older.Spec) {
		return c.Client.Update(context.TODO(), cloudshell)
	}

	return nil
}

// CreateRouteRule create a service resource in the same namespace of cloudshell no matter what expose model.
// if the expose model is ingress or virtualService, it will create additional resources, e.g: ingress or virtualService.
// and the accressUrl will be update.
func (c *Controller) CreateRouteRule(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell, worker *corev1.Pod) (string, error) {
	var accessURL string
	switch cloudshell.Spec.ExposeMode {
	case "", cloudshellv1alpha1.ExposureServiceNodePort:
		// Default(No explicit `ExposeMode` specified in CR) mode is Nodeport
		host, err := c.GetMasterNodeIP(ctx)
		if err != nil {
			klog.ErrorS(err, "unable to get contro plane node IP addr", "cloudshell", klog.KObj(cloudshell))
		}

		service, err := c.CreateCloudShellService(cloudshell, worker)
		if err != nil {
			return "", fmt.Errorf("failed to create service for cloudshell, err: %v", err)
		}

		var nodePort int32
		for _, port := range service.Spec.Ports {
			if port.NodePort != 0 {
				nodePort = port.NodePort
				break
			}
		}
		accessURL = fmt.Sprintf("%s:%d", host, nodePort)
	case cloudshellv1alpha1.ExposureIngress:
		if err := c.CreateIngressForCloudshell(ctx, worker.GetName(), cloudshell); err != nil {
			klog.ErrorS(err, "failed to create ingress for cloudshell", "cloudshell", klog.KObj(cloudshell))
			return "", err
		}

		accessURL = SetRouteRulePath(cloudshell)
	case cloudshellv1alpha1.ExposureVirtualService:
		if err := c.CreateVirtualServiceForCloudshell(ctx, worker.GetName(), cloudshell); err != nil {
			klog.ErrorS(err, "failed to create virtualservice for cloudshell", "cloudshell", klog.KObj(cloudshell))
			return "", err
		}

		accessURL = SetRouteRulePath(cloudshell)
	}

	return accessURL, nil
}

// GetMasterNodeIP could find the one master node IP.
func (c *Controller) GetMasterNodeIP(ctx context.Context) (string, error) {
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

	var internalIP string
	for _, addr := range nodes.Items[0].Status.Addresses {
		// Using External IP as first priority
		if addr.Type == corev1.NodeExternalIP {
			return addr.Address, nil
		}
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
		}
	}
	if len(internalIP) != 0 {
		return internalIP, nil
	}
	return "", nil
}

// CreateCloudShellService Create service resource for cloudshell, the service type is either ClusterIP, NodePort,
// Ingress or virtualService. if the expose model is ingress or virtualService. it will create clusterIP type service.
func (c *Controller) CreateCloudShellService(cloudshell *cloudshellv1alpha1.CloudShell, worker *corev1.Pod) (*corev1.Service, error) {
	serviceBytes, err := util.ParseTemplate(manifests.ServiceTmplV1, struct {
		Name      string
		Namespace string
		Worker    string
		Type      string
	}{
		Name:      fmt.Sprintf("cloudshell-%s", cloudshell.Name),
		Namespace: worker.GetNamespace(),
		Worker:    worker.GetName(),
		Type:      string(corev1.ServiceTypeNodePort),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse cloudshell service manifest")
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(serviceBytes, nil, nil)
	if err != nil {
		klog.ErrorS(err, "failed to decode service manifest", "cloudshell", klog.KObj(cloudshell))
		return nil, err
	}

	svc := obj.(*corev1.Service)
	svc.SetLabels(map[string]string{constants.WorkerOwnerLabelKey: cloudshell.Name})

	// set reference for service, once the cloudshell is deleted, the service is else deleted.
	if err := ctrlutil.SetControllerReference(cloudshell, svc, c.Scheme); err != nil {
		return nil, err
	}

	return svc, util.CreateOrUpdateService(c.Client, svc)
}

// CreateIngressForCloudshell create ingress for cloudshell, if there isn't an ingress controller server
// in the cluster, the ingress is still not working. before create ingress, there's must a service
// as the ingress backend service. all of services should be loaded in an ingress "cloudshell-ingress".
func (c *Controller) CreateIngressForCloudshell(ctx context.Context, service string, cloudshell *cloudshellv1alpha1.CloudShell) error {
	ingress := &networkingv1.Ingress{}
	objectKey := IngressNamespacedName(cloudshell)

	if err := c.Get(ctx, objectKey, ingress); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		var ingressClassName string
		if cloudshell.Spec.IngressConfig != nil && len(cloudshell.Spec.IngressConfig.IngressClassName) > 0 {
			ingressClassName = cloudshell.Spec.IngressConfig.IngressClassName
		}

		// set default path prefix.
		ingressBytes, err := util.ParseTemplate(manifests.IngressTmplV1, struct {
			Name             string
			Namespace        string
			IngressClassName string
			Path             string
			ServiceName      string
		}{
			Name:             objectKey.Name,
			Namespace:        objectKey.Namespace,
			IngressClassName: ingressClassName,
			ServiceName:      service,
			Path:             SetRouteRulePath(cloudshell),
		})
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
		ingress.SetLabels(map[string]string{constants.WorkerOwnerLabelKey: cloudshell.Name})

		return c.Create(ctx, ingress)
	}

	ingressRule := ingress.Spec.Rules[0].IngressRuleValue.HTTP
	ingressBackend := networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: service,
			Port: networkingv1.ServiceBackendPort{
				Number: 7681,
			},
		},
	}
	// if the path already exists in ingress, then update it
	routePath := SetRouteRulePath(cloudshell)
	found := false
	for i := 0; i < len(ingressRule.Paths); i++ {
		if routePath == ingressRule.Paths[i].Path {
			ingressRule.Paths[i].Backend = ingressBackend
			found = true
		}
	}
	// if the path not exists in ingress, then add it
	if !found {
		pathType := networkingv1.PathTypePrefix
		ingressRule.Paths = append(ingressRule.Paths, networkingv1.HTTPIngressPath{
			PathType: &pathType,
			Path:     routePath,
			Backend:  ingressBackend,
		})
	}
	// TODO: All paths will be rewritten here
	ans := ingress.GetAnnotations()
	if ans == nil {
		ans = make(map[string]string)
	}
	ans["nginx.ingress.kubernetes.io/rewrite-target"] = "/"
	ingress.SetAnnotations(ans)
	return c.Update(ctx, ingress)
}

func (c *Controller) CreateServiceFor(_ context.Context, worker *corev1.Pod) error {
	serviceBytes, err := util.ParseTemplate(manifests.ServiceTmplV1, struct {
		Name      string
		Namespace string
		Worker    string
		Type      string
	}{
		Name:      worker.GetName(),
		Namespace: worker.GetNamespace(),
		Worker:    worker.GetName(),
		Type:      string(corev1.ServiceTypeClusterIP),
	})
	if err != nil {
		return errors.Wrap(err, "failed to parse service manifest")
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(serviceBytes, nil, nil)
	if err != nil {
		klog.ErrorS(err, "failed to decode service manifest", "worker", klog.KObj(worker))
		return err
	}

	svc := obj.(*corev1.Service)
	svc.SetLabels(map[string]string{constants.WorkerOwnerLabelKey: worker.Name})

	// set reference for service, once the cloudshell is deleted, the service is else deleted.
	if err := ctrlutil.SetControllerReference(worker, svc, c.Scheme); err != nil {
		return err
	}

	return util.CreateOrUpdateService(c.Client, svc)
}

// CreateVirtualServiceForCloudshell create a virtualService resource in the cluster. if there
// is no istio server be deployed in the cluster. will not create the resource.
func (c *Controller) CreateVirtualServiceForCloudshell(ctx context.Context, service string, cloudshell *cloudshellv1alpha1.CloudShell) error {
	config := cloudshell.Spec.VirtualServiceConfig
	if config == nil {
		return errors.New("unable create virtualservice, missing configuration options")
	}

	// TODO: check the crd in the cluster.

	objectKey := VsNamespacedName(cloudshell)
	virtualService := &istionetworkingv1beta1.VirtualService{}

	if err := c.Get(ctx, objectKey, virtualService); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// if there is not virtualService in the cluster, create the base virtualService.
		vitualServiceBytes, err := util.ParseTemplate(manifests.VirtualServiceV1Beta1, struct {
			Name        string
			Namespace   string
			ExportTo    string
			Gateway     string
			Path        string
			ServiceName string
		}{
			Name:        objectKey.Name,
			Namespace:   objectKey.Namespace,
			ExportTo:    config.ExportTo,
			Gateway:     config.Gateway,
			Path:        SetRouteRulePath(cloudshell),
			ServiceName: service,
		})
		if err != nil {
			return errors.Wrapf(err, "failed to parse cloudshell [%s] virtualservice", cloudshell.Name)
		}

		decoder := serializer.NewCodecFactory(c.Scheme).UniversalDeserializer()
		obj, _, err := decoder.Decode(vitualServiceBytes, nil, nil)
		if err != nil {
			klog.ErrorS(err, "failed to decode virtualservice manifest", "cloudshell", klog.KObj(cloudshell))
			return err
		}
		virtualService = obj.(*istionetworkingv1beta1.VirtualService)
		virtualService.SetLabels(map[string]string{constants.WorkerOwnerLabelKey: cloudshell.Name})

		return c.Create(ctx, virtualService)
	}
	found := false
	httpRoutes := virtualService.Spec.Http
	routePath := SetRouteRulePath(cloudshell)
	// if the path already exists in the virtualService, update Destination.Host
	for i := 0; i < len(httpRoutes); i++ {
		match := httpRoutes[i].Match
		if len(match) > 0 && match[0].Uri != nil {
			if prefix, ok := match[0].Uri.MatchType.(*networkingv1beta1.StringMatch_Prefix); ok &&
				routePath == prefix.Prefix {
				httpRoutes[i].Route[0].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", service, objectKey.Namespace)
				found = true
			}
		}
	}
	// if the path not exists, add a route in the virtualService
	if !found {
		newHTTPRoute := virtualService.Spec.Http[0].DeepCopy()
		newHTTPRoute.Match[0].Uri.MatchType = &networkingv1beta1.StringMatch_Prefix{Prefix: routePath}
		newHTTPRoute.Route[0].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", service, objectKey.Namespace)
		virtualService.Spec.Http = append(virtualService.Spec.Http, newHTTPRoute)
	}

	return c.Update(ctx, virtualService)
}

// UpdateCloudshellStatus update the clodushell status.
func (c *Controller) UpdateCloudshellStatus(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell, phase string) error {
	firstTry := true
	cloudshell.Status.Phase = phase
	status := cloudshell.Status
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		if !firstTry {
			var getErr error
			cloudshell, getErr = c.lister.CloudShells(cloudshell.Namespace).Get(cloudshell.Name)
			if getErr != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
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

func (c *Controller) removeCloudshell(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	worker, err := c.GetBindingWorkerFor(cloudshell)
	if err != nil {
		return err
	}

	if worker != nil {
		if err = c.ResetWorker(ctx, cloudshell); err != nil {
			klog.ErrorS(err, "Failed to reset worker", "cloudshell", cloudshell.Name)
		}
		if err := c.workerPool.Back(worker); err != nil {
			klog.ErrorS(err, "Failed to back worker", "cloudshell", cloudshell.Name)
			return err
		}

		// delete the label to unbind the worker.
		delete(cloudshell.Labels, constants.CloudshellPodLabelKey)
	}

	klog.V(4).InfoS("Delete cloudshell", "cloudshell", klog.KObj(cloudshell))
	if err := c.removeCloudshellRoute(ctx, cloudshell); err != nil {
		return err
	}

	return c.removeFinalizer(cloudshell)
}

// ResetWorker cleanup the kubeConfig and kill ttyd
func (c *Controller) ResetWorker(_ context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
	return execCommand(cloudshell, []string{resetScriptPath}, c.config)
}

// removeCloudshell remove the cloudshell, at the same time, update addition resource.
// i.g: ingress or vitualService. if all of cloudshells was removed, it will delete the
// ingress or vitualService.
func (c *Controller) removeCloudshellRoute(ctx context.Context, cloudshell *cloudshellv1alpha1.CloudShell) error {
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
		newIngressPaths := []networkingv1.HTTPIngressPath{}
		routePath := SetRouteRulePath(cloudshell)
		for i := 0; i < len(ingressRule.Paths); i++ {
			if routePath != ingressRule.Paths[i].Path {
				newIngressPaths = append(newIngressPaths, ingressRule.Paths[i])
			}
		}
		klog.V(4).InfoS("ingress rule remove result", "cloudshell", cloudshell.Name, "old paths count", len(ingressRule.Paths), "new paths count", len(newIngressPaths))
		ingressRule.Paths = newIngressPaths

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

		routePath := SetRouteRulePath(cloudshell)
		// remove rule from virtualService. if the length of virtualService is zero, delete it directly.
		httpRoutes := virtualService.Spec.Http
		newHTTPRoute := []*networkingv1beta1.HTTPRoute{}
		for i := 0; i < len(httpRoutes); i++ {
			match := httpRoutes[i].Match
			if len(match) > 0 && match[0].Uri != nil {
				if prefix, ok := match[0].Uri.MatchType.(*networkingv1beta1.StringMatch_Prefix); ok {
					if routePath != prefix.Prefix {
						newHTTPRoute = append(newHTTPRoute, httpRoutes[i])
					}
				} else {
					newHTTPRoute = append(newHTTPRoute, httpRoutes[i])
				}
			} else {
				newHTTPRoute = append(newHTTPRoute, httpRoutes[i])
			}
		}
		klog.InfoS("virtual service rule remove result", "cloudshell", cloudshell.Name, "old paths count", len(httpRoutes), "new paths count", len(newHTTPRoute))
		if len(newHTTPRoute) == 0 {
			return c.Delete(ctx, virtualService)
		}

		virtualService.Spec.Http = newHTTPRoute
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

// IngressNamespacedName return a namespacedName according to ingressConfig.
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

// VsNamespacedName return a namespacedName according to virtaulServiceConfig.
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

// GenerateKubeconfigInCluster load serviceaccount info under
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

	token, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}

	rootCa, err := os.ReadFile(rootCAFile)
	if err != nil {
		return nil, err
	}

	return util.ParseTemplate(manifests.KubeconfigTmplV1, struct {
		CAData string
		Server string
		Token  string
	}{
		Server: "https://" + net.JoinHostPort(host, port),
		CAData: base64.StdEncoding.EncodeToString(rootCa),
		Token:  string(token),
	})
}

func (c *Controller) GetBindingWorkerFor(cloudshell *cloudshellv1alpha1.CloudShell) (*corev1.Pod, error) {
	podName := cloudshell.Labels[constants.CloudshellPodLabelKey]
	if len(podName) == 0 {
		return nil, nil
	}

	pod, err := c.podLister.Pods(cloudshell.GetNamespace()).Get(podName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return pod, nil
}

func execCommand(cloudshell *cloudshellv1alpha1.CloudShell, command []string, config *rest.Config) error {
	config.GroupVersion = &corev1.SchemeGroupVersion
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.APIPath = "/api"

	clusterClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	options := &exec.ExecOptions{
		Command:   command,
		Executor:  &exec.DefaultRemoteExecutor{},
		Config:    config,
		PodClient: clusterClient.CoreV1(),
		StreamOptions: exec.StreamOptions{
			IOStreams: genericiooptions.IOStreams{
				In:     bytes.NewBuffer([]byte{}),
				Out:    bytes.NewBuffer([]byte{}),
				ErrOut: bytes.NewBuffer([]byte{}),
			},
			Stdin:     false,
			Namespace: cloudshell.Namespace,
			PodName:   cloudshell.Labels[constants.CloudshellPodLabelKey],
		},
	}

	if err := options.Validate(); err != nil {
		return err
	}

	if err := options.Run(); err != nil {
		klog.ErrorS(err, "Failed to run command", "cloudshell", cloudshell.Name, "command", command)
		return err
	}
	return nil
}
