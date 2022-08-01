# cloudtty：一款专用于 Kubernetes 的 Cloud Shell Operator

简体中文 | [English](https://github.com/cloudtty/cloudtty/blob/main/README.md)

cloudtty 是专为 Kubernetes 云原生环境打造的 Web 终端和 Cloud Shell Operator。通过 cloudtty，您可以轻松用浏览器打开一个终端窗口，操控多云资源。

TTY 全称为 TeleTYpe，即电传打字机、电报机。具体而言，TTY 从线缆相连的有线电报机逐渐发展而来，它是一种只负责显示和打字的纯 IO 设备。
近些年随着虚拟化技术的飞速发展，TTY 常指虚拟控制台或虚拟终端。

cloudtty 意指云原生虚拟控制台，也称为 Cloud Shell（云壳）。想象一下，在复杂多变的多云环境中，嵌入这样一层 Shell 来操纵整个云资源。
而这就是 cloudtty 的设计初衷，我们希望在日趋复杂的 Kubernetes 容器云环境中，能够为开发者提供一个简单的 Shell 入口。

## 为什么需要 cloudtty?

目前，社区的 [ttyd](https://github.com/tsl0922/ttyd) 等项目对 TTY 技术的探索已经达到了一定的深度，可以提供浏览器之上的终端能力。

但是在 Kubernetes 的场景下，这些 TTY 项目还需要更加云原生的能力拓展。
如何让 ttyd 在容器内运行，如何通过 NodePort\Ingress 等方式访问，如何用 CRD 的方式创建多个实例？

恭喜你，cloudtty 提供了这些问题的解决方案，欢迎试用 cloudtty 🎉!

## 适用场景

cloudtty 适用于以下几个场景。

1. 如果企业正使用容器云平台来管理 Kubernetes，但由于安全原因，无法随意 SSH 到主机上执行 kubectl 命令，这就需要一种 Cloud Shell 能力。

2. 在浏览器网页上进入运行中的容器(`kubectl exec`)的场景。

3. 在浏览器网页上能够滚动展示容器日志的场景。

cloudtty 的网页终端使用效果如下：

![screenshot_gif](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.gif)

如果将 cloudtty 集成到您自己的 UI 里面，最终效果 demo 如下:

![demo_png](https://github.com/cloudtty/cloudtty/raw/main/docs/demo.png)

## 快速入门步骤

cloudtty 的入门比较简单，请参照以下步骤进行安装和使用。

1. 安装并等待 Pod 运行起来。

  ```
  helm repo add daocloud  https://release.daocloud.io/chartrepo/cloudshell
  helm install cloudtty-operator --version 0.2.0 daocloud/cloudtty
  kubectl wait deployment  cloudtty-operator-controller-manager   --for=condition=Available=True
  ```

2. 创建 CR，启动 cloudtty 的实例，并观察其状态。

  ```
  kubectl apply -f https://raw.githubusercontent.com/cloudtty/cloudtty/v0.2.0/config/samples/local_cluster_v1alpha1_cloudshell.yaml
  ```

  更多范例，参见`config/samples/`。

3. 观察 CR 状态，获取访问接入点：

  ```
  kubectl get cloudshell -w
  ```

  输出类似于：

  ```shell
  NAME                 USER   COMMAND  TYPE        URL                 PHASE   AGE
  cloudshell-sample    root   bash     NodePort    192.168.4.1:30167   Ready   31s
  cloudshell-sample2   root   bash     NodePort    192.168.4.1:30385   Ready   9s
  ```

  当 cloudshell 对象状态变为 `Ready`，并且 `URL` 字段出现之后，就可以通过该字段的访问方式，在浏览器打开:

  ![screenshot_png](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.png)

## 进阶用法

### 进阶 1. 用 cloudtty 访问其他集群

如果是本地集群，可以不提供 kubeconfig，cloudtty 会创建具有 `cluster-admin` 角色权限的 `serviceaccount`。
在容器的内部，`kubectl` 会自动发现 `ca` 证书和 token。如果有安全方面的考虑，您也可以自己提供 kubeconfig 来控制不同用户的权限。

如果是远端集群，cloudtty 可以执行 kubectl 命令行工具。若访问集群，需要指定 kubeconfig。
用户需自己提供 kubeconfig 并储存在 ConfigMap 中，并且在 `cloudshell` 的 CR 中，通过 `spec.configmapName` 指定 ConfigMap 的名称。
cloudtty 会自动挂载到容器中，请确保服务器地址与集群网络连接顺畅。

设置 kubeconfig 的步骤:

1. 准备 `kube.conf`，放入 ConfigMap 中，并确保密钥/证书是 base64 而不是本地文件。

  ```
  kubectl create configmap my-kubeconfig --from-file=kube.config`
  ```

2. 编辑这个 ConfigMap, 修改 endpoint 的地址，从 IP 改为 servicename，如 `server: https://kubernetes.default.svc.cluster.local:443`

### 进阶 2：修改访问方式

cloudtty 提供了以下 4 种服务暴露模式以满足不同的使用场景。

* `ClusterIP`：在集群中创建 ClusterIP 类型的 [Service](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/) 资源。适用于第三方集成 cloudtty 服务，用户可以选择更加灵活的方式来暴露自己的服务。

* `NodePort`：这是默认的模式，也是最简单的暴露服务模式。在集群中创建 NodePort 类型的 Service 资源，通过节点 IP 和对应的端口号访问 cloudtty 服务。

* `Ingress`：在集群中创建 ClusterIP 类型的 Service 资源，并创建 Ingress 资源，通过路由规则负载到 Service 上。
  适合在集群中使用 [Ingress Controller](https://kubernetes.io/zh-cn/docs/concepts/services-networking/ingress-controllers/) 进行流量负载的情况。

* `VirtualService (istio)`：在集群中创建 ClusterIP 类型的 Service 资源，并创建 VirtaulService 资源。适合在集群中使用 [Istio](https://github.com/istio/istio) 进行流量负载的情况。

### 工作原理

cloudtty 的工作原理如下：

1. Operator 会在对应的命名空间（Namespace）下创建同名的 `job` 和 `service`。如果使用 Ingress 或者 VitualService 模式，还会创建对应的路由信息。

2. 当 Pod 运行状态为 `Ready` 之后，就将访问点写入 cloudshell 的 status 里。

3. 当 [Job](https://kubernetes.io/zh-cn/docs/concepts/workloads/controllers/job/) 在 TTL 之后或者因其他原因结束之后，
  一旦 Job 状态变为 `Completed`，cloudshell 的状态也会变为 `Completed`。我们可以设置当 cloudshell 的状态为 `Completed` 时，同时删除相关联的资源。

4. 当 cloudshell 被删除时，会自动删除对应的 Job 和 Service (通过 `ownerReference`), 如果使用 Ingress 或者 VitualService 模式时，还会删除对应的路由信息。

## 开发者模式

cloudtty 还提供了开发者模式。

1. 运行 Operator 并安装 CRD。由开发者进行编译执行（建议普通用户使用上述 [Helm 安装](#快速上手)）。

      1. 安装 CRD

        - （Option 1）从 YAML：先 `make generate-yaml`，然后 apply 生成的 yaml

        - （Option 2）从代码：克隆代码之后运行 `make install`

      2. 运行 Operator：`make run`

2. 创建 CR。比如开启窗口后自动打印某个容器的日志：

  ```yaml
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

### 开发指南

基于 kubebuilder 框架开发；以 ttyd 为基础构建镜像。

1. 初始化框架，初始化 kubebuilder 项目。

  ```
  kubebuilder init --domain daocloud.io --repo daocloud.io/cloudshell
  kubebuilder create api --group cloudshell --version v1alpha1 --kind CloudShell
  ```

2. 生成 manifest。

  ```
  make manifests
  ```

3. 调试（使用默认的 kube.conf）。

  ```
  make install # 在目标集群安装 CRD
  make run     # 启动 Operator 的代码
  ```

4. 构建镜像。

  ```
  make docker-build
  make docker-push
  ```

5. 生成 Operator 部署的 yaml。使用 kustomize 渲染 CRD yaml。

  ```
  make generate-yaml
  ```

6. 构建 helm 包。

  ```
  make build-chart
  ```

> 开发注意事项：
>
> go get 的 gotty 暂不能用，需要下载：
> ```
> wget https://github.com/yudai/gotty/releases/download/v1.0.1/gotty_linux_amd64.tar.gz
> ```

### Docker 镜像和用 Docker 做简化实验

镜像在 master 分支的 docker/目录下，具体操作步骤如下：

1. 创建 ConfigMap 或 kube.conf

  ```
  kubectl create configmap my-kubeconfig --from-file=/root/.kube/config
  ```

2. 根据不同场景操作：

  * 日常 kubectl 的 console。

    ```
    bash run.sh
    ```

  * 实时查看 Event。

    ```
    bash run.sh "kubectl get event -A -w"
    ```

  * 实时查看 Pod 日志。

    ```
    NS=caas-system
    POD=caas-api-6d67bfd9b7-fpvdm
    bash run.sh "kubectl -n $NS logs -f $POD"
    ```

## 特别鸣谢

cloudtty 这个项目的很多技术实现基于 [ttyd](https://github.com/tsl0922/ttyd), 非常感谢 `tsl0922`、`yudai` 和社区开发者们的努力.

cloudtty 前端 UI 及其镜像内所用的二进制文件均源于 ttyd 社区。

## 交流和探讨

如果您有任何疑问，请联系我们：

* [Slack 频道](https://cloud-native.slack.com/archives/C03LA6AUF7V)
* 微信交流群: 请联系 `calvin0327`(wen.chen@daocloud.io) 加入交流群

非常欢迎大家[提出 issue](https://github.com/cloudtty/cloudtty/issues) 和[发起 PR](https://github.com/cloudtty/cloudtty/pulls)。🎉🎉🎉

## 下一步

cloudtty 还将提供更多的功能，此处列出一些已经排上日程的开发计划。

1. 通过 RBAC 生成的 `/var/run/secret` 进行权限控制
2. 代码还未做边界处理（如 NodePort 准备工作）
3. 为了安全, Job 应该在单独的 Namespace 跑，而不是在 CR 中用同一个 Namespace
4. 需要检查 Pod 的 Running 和 endpoint 的 Ready，才能置 CR 为 Ready
5. 目前 TTL 只反映到 shell 的 timeout, 没有反映到 Job 的 yaml 里
6. Job 的创建模板目前是 hardcode 方式，应该提供更灵活的方式修改 Job 的模板
