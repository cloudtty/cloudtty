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
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cloudshellv1alpha1 "gitlab.daocloud.cn/ndx/webtty/pkg/apis/cloudshell/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CloudShellReconciler reconciles a CloudShell object
type CloudShellReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	jobTemplate string
	svcTemplate string
}

//+kubebuilder:rbac:groups=cloudshell.daocloud.io,resources=cloudshells,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cloudshell.daocloud.io,resources=cloudshells/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cloudshell.daocloud.io,resources=cloudshells/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CloudShell object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *CloudShellReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	//reference : https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/cronjob-tutorial/testdata/project/controllers/cronjob_controller.go
	instance := cloudshellv1alpha1.CloudShell{}
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		log.Info("unable to fetch instance. it should be a DELETE event", "msg", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if instance.Status.Phase == "Completed" {
		log.Info("find a completed instance", "instance", instance)
		return ctrl.Result{}, nil
	}
	// Get downstream job and status
	var job *kbatch.Job
	var err error
	job, err = r.get_created_job(ctx, req.Namespace, &instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	if job == nil {
		if err := r.createCloudShellJob(ctx, &instance); err != nil {
			return ctrl.Result{}, err
		}
		instance.Status.Phase = "JobCreated"
		if err := r.Status().Update(ctx, &instance); err != nil {
			log.Error(err, "unable to update instance status")
			return ctrl.Result{}, err
		}
		//Job创建之后，等待job的下一个状态, 等待3s Requeue
		log.Info("Job创建之后，等待job的下一个状态, 等待2s Requeue")
		return ctrl.Result{RequeueAfter: time.Duration(2) * time.Second}, nil

	} else {
		_, finishedType := isJobFinished(job)
		switch finishedType {
		case kbatch.JobFailed:
			instance.Status.Phase = "Failed"
			if err := r.Status().Update(ctx, &instance); err != nil {
				log.Error(err, "unable to update instance status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil // Not Requeue

		case kbatch.JobComplete:
			instance.Status.Phase = "Completed"
			if err := r.Status().Update(ctx, &instance); err != nil {
				log.Error(err, "unable to update instance status")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil // Not Requeue
		case "": // ongoing
			if job.Status.Active == 1 {
				// Get create svc
				var svc *corev1.Service
				svc, err = r.get_created_svc(ctx, req.Namespace, &instance)
				if err != nil {
					return ctrl.Result{}, err
				}
				if svc == nil {
					if err := r.createCloudShellService(ctx, &instance); err != nil {
						log.Error(err, "unable to create service")
						return ctrl.Result{}, err
					}
					instance.Status.Phase = "SvcCreated"
					if err := r.Status().Update(ctx, &instance); err != nil {
						log.Error(err, "unable to update instance status")
						return ctrl.Result{}, err
					}
				} else {
					nodePort := svc.Spec.Ports[0].NodePort
					//FIXME, nodePort may be blank dueto delay filled by k8s, should `ctrl.Result{RequeueAfter: 5}, nil`
					if true {
						// FIXME, job active does not mean pod is running and service endpoint is filled
						instance.Status.Phase = "Ready"
						instance.Status.AccessURL = "NodeIP:" + fmt.Sprintf("%d", nodePort)
						if err := r.Status().Update(ctx, &instance); err != nil {
							log.Error(err, "unable to update instance status")
							return ctrl.Result{}, err
						}
						//等各种服务都ready之后，等待job的Completed, 等待每10s Requeue
						var interval int32
						interval = 10
						//Find a modest interval
						if instance.Spec.Ttl > interval*10 {
							interval = instance.Spec.Ttl / 10
						}
						syncInterval := time.Duration(interval) * time.Second
						log.Info("等待job的Completed", "job", job.Name)
						return ctrl.Result{RequeueAfter: syncInterval}, nil

					}
				}
			} else {
				log.Info("Waiting job to active")
				return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
			}
		}
	}

	return ctrl.Result{}, nil

}

//////////////////////////////
// 获取下属的job资源，根据label “ownership”
//////////////////////////////
func (r *CloudShellReconciler) get_created_job(ctx context.Context, namespace string, owner *cloudshellv1alpha1.CloudShell) (*kbatch.Job, error) {
	log := log.FromContext(ctx)

	var childJobs kbatch.JobList
	//find job in the ns, match label "ownership: parent-CR-name "
	if err := r.List(ctx, &childJobs, client.InNamespace(namespace), client.MatchingLabels{"ownership": owner.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return nil, err
	}
	//log.Info("DEBUG, Found Jobs :", "jobs", len(childJobs.Items))
	if len(childJobs.Items) > 1 {
		err := errors.New("found duplicated child jobs")
		log.Error(err, "more than 1 jobs found")
		return nil, err
	}
	if len(childJobs.Items) == 0 {
		log.Info("job creation is still pending")
		return nil, nil
	}
	theJob := &childJobs.Items[0]
	//log.Info("find created job ", "job.status", theJob.Status)

	return theJob, nil
}

//////////////////////////////
// 获取下属的service资源，根据label “ownership”
//////////////////////////////
func (r *CloudShellReconciler) get_created_svc(ctx context.Context, namespace string, owner *cloudshellv1alpha1.CloudShell) (*corev1.Service, error) {
	log := log.FromContext(ctx)

	var childSvcs corev1.ServiceList
	if err := r.List(ctx, &childSvcs, client.InNamespace(namespace), client.MatchingLabels{"ownership": owner.Name}); err != nil {
		log.Error(err, "unable to list child svc")
		return nil, err
	}
	//log.Info("DEBUG, Found Svc :", "svcs", len(childSvcs.Items))
	if len(childSvcs.Items) > 1 {
		err := errors.New("found duplicated child svcs")
		log.Error(err, "more than 1 svcs found")
		return nil, err
	}
	if len(childSvcs.Items) == 0 {
		log.Info("svc creation is still pending")
		return nil, nil
	}
	theSvc := &childSvcs.Items[0]
	//log.Info("find created svc", "nodeport", theSvc.Spec.Ports[0].NodePort)

	return theSvc, nil
}

/////////////////////////////
// 加载YAML模板
////////////////////////////
func (r *CloudShellReconciler) loadYamlTemplate(fname string) (str string, err error) {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return "", err
	}
	str = string(b)
	return str, nil
}

/////////////////////////////
// 创建Job
////////////////////////////
func (r *CloudShellReconciler) createCloudShellJob(ctx context.Context, inst *cloudshellv1alpha1.CloudShell) error {
	log := log.FromContext(ctx)
	var err error
	if r.jobTemplate == "" {
		r.jobTemplate, err = r.loadYamlTemplate("job.yaml.tmpl")
		if err != nil {
			log.Error(err, "Failed to loadJobTemplate")
			return err
		}
	}
	//https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, groupVersionKind, err := decoder.Decode([]byte(r.jobTemplate), nil, nil)
	log.Info("")
	if err != nil {
		log.Error(err, "Error while decoding YAML object. Err was: ")
		return err
	}
	log.Info("Print gvk", "gvk", groupVersionKind)
	job := obj.(*kbatch.Job)
	job.Name = "job-" + inst.Name
	job.Namespace = inst.Namespace
	job.Labels["ownership"] = inst.Name
	container := &job.Spec.Template.Spec.Containers[0]
	container.Env = append(container.Env, corev1.EnvVar{Name: "COMMAND", Value: inst.Spec.CommandAction})
	container.Env = append(container.Env, corev1.EnvVar{Name: "TTL", Value: fmt.Sprintf("%d", inst.Spec.Ttl)})
	container.Env = append(container.Env, corev1.EnvVar{Name: "ONCE", Value: strconv.FormatBool(inst.Spec.Once)})
	job.Spec.Template.Spec.Volumes[0].ConfigMap.Name = inst.Spec.ConfigmapName
	//设置子资源的owner reference, 就可以级联删除
	if err := ctrlutil.SetControllerReference(inst, job, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference")
		return err
	}
	log.Info("Print job", "job", job)
	err = r.Create(ctx, job)
	if err != nil {
		log.Error(err, "Error during Create instance")
		return err
	}
	return nil
}

////////////////////////////
// 创建service 资源
///////////////////////////
func (r *CloudShellReconciler) createCloudShellService(ctx context.Context, inst *cloudshellv1alpha1.CloudShell) error {
	log := log.FromContext(ctx)
	var err error
	if r.svcTemplate == "" {
		r.svcTemplate, err = r.loadYamlTemplate("service.yaml.tmpl")
		if err != nil {
			log.Error(err, "Failed to load Yaml Template")
			return err
		}
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(r.svcTemplate), nil, nil)
	if err != nil {
		log.Error(err, "Error while decoding YAML object. Err was: ")
		return err
	}
	svc := obj.(*corev1.Service)
	svc.Name = "svc-" + inst.Name
	svc.Namespace = inst.Namespace
	svc.Labels["ownership"] = inst.Name
	svc.Spec.Selector["job-name"] = "job-" + inst.Name
	//设置子资源的owner reference, 就可以级联删除
	if err := ctrlutil.SetControllerReference(inst, svc, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference")
		return err
	}
	log.Info("Print svc", "svc", svc)
	err = r.Create(ctx, svc)
	if err != nil {
		log.Error(err, "Error during Create svc resource")
		return err
	}
	return nil
}

//////////////////////////////
func isJobFinished(job *kbatch.Job) (bool, kbatch.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}

	return false, ""
}

////////////////////////////
// SetupWithManager sets up the controller with the Manager.
func (r *CloudShellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cloudshellv1alpha1.CloudShell{}).
		Complete(r)
}
