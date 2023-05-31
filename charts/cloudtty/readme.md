## Introduction

cloudtty is an easy-to-use operator to run web terminal and cloud shell intended for a kubernetes-native environment. more information [CloudTTY](https://github.com/cloudtty/cloudtty/blob/main/README.md)

## Install

First, add the CloudTTY chart repo to your local repository.
``` bash
$ helm repo add cloudtty https://release.daocloud.io/chartrepo/cloudtty

$ helm repo list
NAME          	URL
cloudtty        https://cloudtty.github.io/cloudtty
```

With the repo added, available charts and versions can be viewed.
``` bash
$ helm search repo cloudtty
```

You can run the following command to install the CloudTTY Operator and CRDs
``` bash
$ export version=0.5.0
$ helm install cloudtty-operator --version ${version} cloudtty/cloudtty 
```

Wait for the operator pod until it is running.
``` bash
kubectl wait deployment cloudtty-operator-controller-manager --for=condition=Available=True
```

## Usage
Create a CloudTTY instance by applying CR, and then monitor its status
``` bash
kubectl apply -f https://raw.githubusercontent.com/cloudtty/cloudtty/v${version}/config/samples/local_cluster_v1alpha1_cloudshell.yaml
```

Observe CR status to obtain its web access url, such as:
``` bash
kubectl get cloudshell -w
```
You can see the following information:
``` bash
  NAME                 USER   COMMAND  TYPE        URL                 PHASE   AGE
  cloudshell-sample    root   bash     NodePort    192.168.4.1:30167   Ready   31s
```

## Uninstall

If CloudTTY related custom resources already exist, you need to clear.
``` bash
kubectl delete cloudshells.cloudshell.cloudtty.io --all
```

Uninstall CloudTTY components via helm.
``` bash
$ helm uninstall cloudtty-operator 
$ kubectl delete crd clusteroperations.kubean.io
```

## Parameters

The following table lists the configurable parameters of the CloudTTY chart and their default values.

| Key | Type | Default | Describe |
| --- | ---- | ------- | -------- |
| global.imageRegistry | string | "" | Global Docker image registry |
| global.imagePullSecrets | list | [] | Specify Docker-registry secret names as an array |
| installCRDs | bool | true | Define flag whether to install CRD resources |
| labels | dict | {} | Controller Manager labels |
| replicaCount | int | 1 | Target replicas |
| podAnnotations | dict | {} | Controller Manager pod annotations |
| podLabels | dict | {} | Controller Manager pod labels |
| image.registry | string | ghcr.io | Cloudtty image registry |
| image.repository | string | cloudtty/cloudshell-operator | Cloudtty image repository |
| image.tag | string | v0.5.3 | Cloudtty image tag (immutable tags are recommended) |
| image.pullPolicy | string | IfNotPresent | Cloudtty image pull policy |
| image.pullSecrets | list | [] | Specify Docker-registry secret names as an array |
| resources | dict | {} | Resources |
| nodeSelector | dict | {} | Controller Manager node selector |
| affinity | dict | {} | Controller Manager affinity |
| tolerations | list | {} | Controller Manager tolerations |
| livenessProbe.enabled | bool | true | Enable liveness Probe on Kafka containers |
| livenessProbe.initialDelaySeconds | int | 15 | Initial delay seconds for liveness Probe |
| livenessProbe.periodSeconds | int | 20 | Period seconds for liveness Probe |
| readinessProbe.enabled | bool | true | Enable readiness Probe on Kafka containers |
| readinessProbe.initialDelaySeconds | int | 5 | Initial delay seconds for readiness Probe |
| readinessProbe.periodSeconds | int | 10 | Period seconds for readiness Probe |
| featureGates.AllowSecretStoreKubeconfig | bool | false | Allow Secret Store Kubeconfig is a feature gate for the cloudshell to store kubeconfig in secret |
| jobTemplate.labels | dict | {} | Job Template labels |
| jobTemplate.image.registry | string | ghcr.io | Cloudtty Job image registry |
| jobTemplate.image.repository | string | cloudtty/cloudshell | Cloudtty Job image repository |
| jobTemplate.image.tag | string | v0.5.3 | Cloudtty Job image tag (immutable tags are recommended) |
| jobTemplate.image.pullPolicy | string | IfNotPresent | Cloudtty Job image pull policy |
| jobTemplate.image.pullSecrets | list | [] | Specify Docker-registry secret names as an array |
