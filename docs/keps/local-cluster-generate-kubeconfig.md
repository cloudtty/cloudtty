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

### Create RBAC resources for cloudshell on local cluster

1. we can leverage an existing clusterrole `cluster-admin`, it has the highest privileges:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: cluster-admin
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'
```

2. create `serviceaccount` for cloudshell and binding to a new resource `cluterrolebinding`. e.g:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: cloudtty-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: ServiceAccout
  name: cloudtty-admin
```

### Use serviceaccount running cloudshell job

According to the [documentation](https://kubernetes.io/docs/reference/kubectl/#in-cluster-authentication-and-namespace-overrides), we only need to run the job using serviceaacount `cloudtty-admin`. `kubectl` tool will automatically detect that it is running inside the cluster and load the `caData` and `token` under directory `/var/run/secrets/kubernetes.io/serviceaccount`. the namespace from `/var/run/secrets/kubernetes.io/serviceaccount/namespace` as the default namespace what it is the namespace of the serviceAccount. we can modify the default namespace by specifying the `POD_NAMESPACE` environment variable.

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  namespace: {{ .Namespace }}
  name: {{ .Name }}
  labels:
    ownership: {{ .Ownership }}
spec:
  activeDeadlineSeconds: 3600
  ttlSecondsAfterFinished: 60
  parallelism: 1
  completions: 1
  template:
    spec:
      serviceAccount: cloudtty-admin
      containers:
      - name:  web-tty
        image: ghcr.io/cloudtty/cloudshell:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 7681
          name: tty-ui
          protocol: TCP
        command:
          - bash
          - "-c"
          - |
            once=""
            index=""
            if [ "${ONCE}" == "true" ];then once=" --once "; fi;
            if [ -f /index.html ]; then index=" --index /index.html ";fi
            if [ -z "${TTL}" ] || [ "${TTL}" == "0" ];then
                ttyd ${index} ${once} sh -c "${COMMAND}"
            else
                timeout ${TTL} ttyd ${index} ${once} sh -c "${COMMAND}" || echo "exiting"
            fi
        env:
        - name: TTL
          value: "{{ .Ttl }}"
        - name: COMMAND
          value: {{ .Command }}
        - name: POD_NAMESPACE
          value: default
```

