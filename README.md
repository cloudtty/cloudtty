```
#init kubebuilder project
kubebuilder init --domain daocloud.io --repo daocloud.io/cloudshell
kubebuilder create api --group cloudshell --version v1alpha1 --kind CloudShell
```

stage 2:
```
make manifests

```
