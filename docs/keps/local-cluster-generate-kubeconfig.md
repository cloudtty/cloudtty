---
short-desc: Genetate kubeconfig for local cluster
title: Genetate Kubeconfig For Local Cluster
authors:
  - "@calvin0327"
reviewers:
  - "@panpan0000"
approvers:
  - ""
creation-date: 2022-06-16
last-updated: 2022-06-17
---

# Generate kubeconfig for local cluster

## Table of Contents

* [Generate kubeconfig for local cluster](#generate-kubeconfig-for-local-cluster)
  * [Table of Contents](#table-of-contents)
  * [Motivation](#motivation)
    * [Goals](#goals)
  * [Implementation Details](#implementation-details)

This document proposes a mechanism to generate kubeconfig by serviceaccount while we want to create a cloudshell to manage clusters deployed with cloudshell-operator.

## Motivation

If we want to create a cloud shell for cluster that the `cloudshell-operator` is depoly on it. this is what we called local cluster. it's not friendly for users to generate kubeconfig and mount to pod of cloudshell manually. all of actions can be done by `cloudshell-operator` for users.

### Goals

* It can help users to generate kubeconfig automatically and mount it to pod of cloudshell, it's normal to use kubectl command to operate the local cluster. 
* The user corresponding to kubeconfig has `admin` privileges.

## Implementation details

User can not support configmap info to field `spec.configmap` of cloudshell cr. If this field is empty and we assume that the user want to created cloudshell in the local cluster. `cloudshell-operator` will complete the following steps:

### Grant operator to admin permission

1. We need to use operator ServiceAccount to generate kubeconfig, so operator needs to have sufficient permissions.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cloudtty-controller-manager
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
```

2. Inside the cluster, use ServiceAccount to generate kubeconfig, load the `caData` and `token` under directory `/var/run/secrets/kubernetes.io/serviceaccount`. `Server` consists of the KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variables.

```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .CAData }}
    server: {{ .Server }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: cloudtty-controller-manager
  name: cloudtty-controller-manager@kubernetes
current-context: cloudtty-controller-manager@kubernetes
kind: Config
users:
- name: cloudtty-controller-manager
  user: 
    token: {{ .Token }}
```

### restore configmap for kubeconfig and mount to the job

We will restore kubeconfig to configmap in the same namespace with cloudshell, and mount the configmap to pod of job and backfill configMap name to Cloudshell.

