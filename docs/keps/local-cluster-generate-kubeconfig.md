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
last-updated: 2023-01-29
---

# Generate kubeconfig for a local cluster

## Table of Contents

* [Generate kubeconfig for a local cluster](#generate-kubeconfig-for-a-local-cluster)
  * [Table of Contents](#table-of-contents)
  * [Motivation](#motivation)
    * [Goals](#goals)
  * [Implementation](#implementation)
    * [Grant operator to admin permission](#grant-operator-to-admin-permission)
    * [Restore configmap for kubeconfig and mount to the job](#restore-configmap-for-kubeconfig-and-mount-to-the-job)

This document proposes a mechanism to generate kubeconfig by ServiceAccount while you want to create a cloudshell to manage clusters deployed with `cloudshell-operator`.

## Motivation

If you want to create a cloud shell for a cluster that the `cloudshell-operator` is depolyed on it.
This is what we called local cluster. It's not very friendly to generate kubeconfig and mount it to pod of cloudshell manually.
Now, all of actions can be done by `cloudshell-operator`.

### Goals

* Help to generate kubeconfig automatically and mount it to pod of cloudshell. It's normal to use kubectl commands to operate a local cluster.
* Grant `admin` privileges to the kubeconfig user.

## Implementation

The configmap info might not be supported in the field `spec.configmap` of cloudshell cr.
If this field is empty and assume that you want to create cloudshell in a local cluster.
`cloudshell-operator` will complete the following steps:

### Grant operator to admin permission

1. If you use operator ServiceAccount to generate kubeconfig, the operator needs to have sufficient permissions.

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

2. In the cluster, use ServiceAccount to generate kubeconfig, and load the `caData` and `token` into directory `/var/run/secrets/kubernetes.io/serviceaccount`.
   `Server` consists of the KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variables.

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

### Restore configmap for kubeconfig and mount to the job

Restore kubeconfig to configmap in the same namespace with cloudshell, mount the configmap to pod of job, and backfill the configmap name to cloudshell.
