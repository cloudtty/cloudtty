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

### Global configuration

| Name                      | Description                      | Value |
| ------------------------- | -------------------------------- | ----- |
| `global.imageRegistry`    | Global Docker image registry     | `""`  |
| `global.imagePullSecrets` | Global Docker image pull secrets | `[]`  |
| `global.imageRegistry`    | Global Docker image registry     | `""`  |
| `global.imagePullSecrets` | Global Docker image pull secrets | `[]`  |

### Pre-Hook configuration

| Name          | Description                                  | Value  |
| ------------- | -------------------------------------------- | ------ |
| `installCRDs` | define flag whether to install CRD resources | `true` |

### controllerManager configuration

| Name                         | Description                                               | Value                          |
| ---------------------------- | --------------------------------------------------------- | ------------------------------ |
| `labels`                     | controllerManager labels                                  | `{}`                           |
| `replicaCount`               | controllerManager target replica count                    | `1`                            |
| `podAnnotations`             | controllerManager pod annotations                         | `{}`                           |
| `podLabels`                  | controllerManager pod labels                              | `{}`                           |
| `coreWorkerLimit`            | defines the core limit of worker pool                     | `nil`                          |
| `maxWorkerLimit`             | defines the max limit of worker pool                      | `nil`                          |
| `scaleInWorkerQueueDuration` | defines the duration (in minutes) to scale in the workers | `nil`                          |
| `image.registry`             | cloudtty image registry                                   | `ghcr.io`                      |
| `image.repository`           | cloudtty image repository                                 | `cloudtty/cloudshell-operator` |
| `image.tag`                  | cloudtty image tag (immutable tags are recommended)       | `v0.8.2`                       |
| `image.pullPolicy`           | cloudtty image pull policy                                | `IfNotPresent`                 |
| `image.pullSecrets`          | Specify docker-registry secret names as an array          | `[]`                           |
| `resources`                  | controllerManager resource requests and limits            | `{}`                           |
| `nodeSelector`               | controllerManager node labels for pod assignment          | `{}`                           |
| `affinity`                   | controllerManager affinity settings                       | `{}`                           |
| `tolerations`                | controllerManager tolerations for pod assignment          | `{}`                           |
| `livenessProbe.enabled`      | Enable livenessProbe on Kafka containers                  | `false`                        |
| `readinessProbe.enabled`     | Enable readinessProbe on Kafka containers                 | `false`                        |
| `cloudshellImage.registry`   | cloudshell image registry                                 | `ghcr.io`                      |
| `cloudshellImage.repository` | cloudshell image repository                               | `cloudtty/cloudshell`          |
| `cloudshellImage.tag`        | cloudshell image tag (immutable tags are recommended)     | `v0.8.2`                       |
