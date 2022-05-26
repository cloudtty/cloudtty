# 这是一个 cloudshell 的 operator

简体中文 | [英文](https://github.com/cloudtty/cloudtty/blob/main/README.md)

# 为什么需要 cloudtty ?

像 [ttyd](https://github.com/tsl0922/ttyd) 等项目已经非常成熟了，可以提供浏览器之上的终端的能力。

但是在 kubernetes 的场景下，我们需要能有更云原生的能力拓展:

比如 ttyd 在容器内运行，能够通过 NodePort\Ingress 等方式访问，能够用 CRD 的方式创建多个实例。

cloudtty 提供了这些功能，请使用 cloudtty 吧🎉!

# 适用场景

1. 很多企业使用容器云平台来管理 Kubernetes, 但是由于安全原因，无法随意 SSH 到主机上执行 kubectl 命令，就需要一种 Cloud Shell 的能力。

2. 在浏览器网页上能够进入运行中的容器(`kubectl exec`)的场景。

3. 在浏览器网页上能够滚动展示容器日志的场景。

# 截图

![screenshot_gif](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.gif)

# 快速上手

步骤1. 安装

	helm repo add daocloud  https://release.daocloud.io/chartrepo/cloudshell
	helm install --version 0.1.0 daocloud/cloudtty --generate-name

步骤2. 准备`kube.conf`,放入 configmap 中

    注：ToDo: 当目标集群跟 operator 是同一个集群则不需要`kube.conf`，会尽快优化

    - （第1步）
	`kubectl create configmap my-kubeconfig --from-file=/root/.kube/config`, 并确保密钥/证书是 base64 而不是本地文件

    - （第2步）
	编辑这个 configmap, 修改 endpoint 的地址，从 IP 改为 servicename, 如`server: https://kubernetes.default.svc.cluster.local:443`

步骤3. 创建CR，启动 cloudtty 的实例，并观察其状态

	kubectl apply -f ./config/samples/cloudshell_v1alpha1_cloudshell.yaml

更多范例，参见`config/samples/`

步骤4. 观察 CR 状态，获取访问接入点，如: 

	$kubectl get cloudshell -w

可以看到：

	NAME                 USER   COMMAND  TYPE        URL                 PHASE   AGE
	cloudshell-sample    root   bash     NodePort    192.168.4.1:30167   Ready   31s
	cloudshell-sample2   root   bash     NodePort    192.168.4.1:30385   Ready   9s

当 cloudshell 对象状态变为`Ready`，并且`URL`字段出现之后，就可以通过该字段的访问方式，在浏览器打开，如下:

![screenshot_png](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.png)

# 访问方式

Cloudtty 提供了4种模式来暴露后端的服务: `ClusterIP`, `NodePort`, `Ingress`, `VitualService`来满足不同的使用场景：

* ClusterIP： 默认的模式，在集群中创建 ClusterIP 类型的 [Service](https://kubernetes.io/docs/concepts/services-networking/service/) 资源。适用于第三方集成 cloudtty 服务，用户可以选择更加灵活的方式来暴露自己的服务。

* NodePort：最简单的暴露服务模式，在集群中创建 NodePort 类型的 Service 资源。可以用过节点 IP 和 对应的端口号访问 cloudtty 服务。

* Ingress：在集群中创建 ClusterIP 类型的 Service 资源，并创建 Ingress 资源，通过路由规则负载到 Service 上。适用于集群中使用 [Ingress Controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) 进行流量负载的情况。

* VirtualService：在集群中创建 ClusterIP 类型的 Service 资源，并创建 VirtaulService 资源。适用于集群中使用 [Istio](https://github.com/istio/istio) 进行流量负载的情况。

#### 原理

1. Operator 会在对应的 NS 下创建同名的 `job` 和`service`，如果使用 Ingress 或者 VitualService 模式时，还会创建对应的路由信息。

2. 当 pod 运行 `Ready` 之后，就将访问点写入 cloudshell 的 status 里。

3. 当 [job](https://kubernetes.io/docs/concepts/workloads/controllers/job/) 在 TTL 或者其他原因结束之后，一旦 job 变为 Completed，cloudshell 的状态也会变`Completed`。我们可以设置当 cloudshell 的状态为 `Completed` 时，同时删除相关联的资源。

4. 当 cloudshell 被删除时，会自动删除对应的 job 和 service (通过`ownerReference`), 如果使用 Ingress 或者 VitualService 模式时，还会删除对应的路由信息。

# 特别鸣谢

这个项目的很多技术实现都是基于`https://github.com/tsl0922/ttyd`, 非常感谢 `tsl0922` `yudai`和社区.

前端 UI 也是从 `ttyd` 项目衍生出来的，另外镜像内所使用的`ttyd`二进制也是来源于这个项目。

### 开发者模式

1. 运行 operator 和安装 CRD

  开发者: 编译执行 （建议普通用户使用上述 Helm 安装）

      b.1 ) 安装CRD
        - （选择1）从YAML： 	   ```make generate-yaml ;              然后apply 生成的yaml```
        - （选择2）从代码：克隆代码之后 `make install`
      b.2 ) 运行Operator :        `make run`

2. 创建 CR 

比如开启窗口后自动打印某个容器的日志：

```
apiVersion: cloudshell.cloudtty.io/v1alpha1
kind: CloudShell
metadata:
  name: cloudshell-sample
spec:
  configmapName: "my-kubeconfig"
  runAsUser: "root"
  commandAction: "kubectl -n kube-system logs -f kube-apiserver-cn-stack"
  once: false
```

ToDo：

- （1）通过 RBAC 生成的/var/run/secret，进行权限控制
- （2）代码中边界处理（如 nodeport 准备好）还没有处理
- （3）为了安全, job 应该在单独的 NS 跑，而不是在 CR 同 NS
-  (4) 需要检查 pod 的 Running 和 endpoint 的 Ready，才能置为 CR 为 Ready
-  (5) 目前 TTL 只反映到 shell 的 timeout, 没有反映到 job 的 yaml 里
-  (6) job 的创建模版目前是 hardcode 方式，应该提供更灵活的方式修改 job 的模版

# 开发指南

基于 kubebuilder 框架开发
基于 ttyd 为基础，构建镜像

1. 初始化框架
```
#init kubebuilder project
kubebuilder init --domain daocloud.io --repo daocloud.io/cloudshell
kubebuilder create api --group cloudshell --version v1alpha1 --kind CloudShell
```

2. 生成manifest
```
make manifests
```

3. 如果调试（使用默认的kube.conf）
```
# DEBUG work
make install # 目标集群 安装CRD
make run     # 启动operator的代码
```

4. 如何构建镜像
```
#build
make docker-build
make docker-push
```

5. 生成 operator 部署的 yaml
```
#use kustomize to render CRD yaml
make generate-yaml
```

6.构建 helm 包
```
make build-chart
```

#开发注意：

#go get的gotty不能用...要下载
wget https://github.com/yudai/gotty/releases/download/v1.0.1/gotty_linux_amd64.tar.gz

# Docker镜像和用docker做简化实验

镜像在master分支的docker/目录下

# 步骤

1. 创建kube.conf

```
# 1.create configmap of kube.conf
kubectl create configmap my-kubeconfig --from-file=/root/.kube/config
```

2.根据不同场景

a) 日常kubectl的console
```
bash run.sh
```

b) 实时查看event
```
bash run.sh "kubectl get event -A -w"
```

c) 实时查看 pod 日志
```
NS=caas-system
POD=caas-api-6d67bfd9b7-fpvdm
bash run.sh "kubectl -n $NS logs -f $POD"
```

我们还会提供更多的功能，非常欢迎大家的 [issue](https://github.com/cloudtty/cloudtty/issues) 和 PR。🎉🎉🎉
