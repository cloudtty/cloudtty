# CloudTTY：一款专用于 Kubernetes 的 Cloud Shell Operator

简体中文 | [English](https://github.com/cloudtty/cloudtty/blob/main/README.md)

CloudTTY 是专为 Kubernetes 云原生环境打造的 Web 终端和 Cloud Shell Operator。
通过 CloudTTY，您可以轻松用浏览器打开一个终端窗口，操控多云资源。

CloudTTY 意指云原生虚拟控制台，也称为 Cloud Shell（云壳）。
想象一下，在复杂多变的多云环境中，嵌入这样一层 Shell 来操纵整个云资源。
而这就是 CloudTTY 的设计初衷，我们希望在日趋复杂的 Kubernetes 容器云环境中，能够为开发者提供一个简单的 Shell 入口。

> TTY 全称为 TeleTYpe，即电传打字机、电报机。
> 没错，就是那种很古老会发出滴滴答答声响、用线缆相连的有线电报机，那是一种只负责显示和打字的纯 IO 设备。
> 近些年随着虚拟化技术的飞速发展，TTY 常指虚拟控制台或虚拟终端。

<img src="https://github.com/cncf/artwork/blob/main/other/illustrations/ashley-mcnamara/transparent/cncf-cloud-gophers-transparent.png?raw=true" style="width:600px;" />

**CloudTTY 是一个[云原生计算基金会 (CNCF)](https://cncf.io/) 全景图项目。**

## 为什么需要 CloudTTY?

目前，社区的 [ttyd](https://github.com/tsl0922/ttyd) 等项目对 TTY 技术的探索已经达到了一定的深度，可以提供浏览器之上的终端能力。

但是在 Kubernetes 的场景下，这些 TTY 项目还需要更加云原生的能力拓展。
如何让 ttyd 在容器内运行，如何通过 NodePort\Ingress 等方式访问，如何用 CRD 的方式创建多个实例？

恭喜你，CloudTTY 提供了这些问题的解决方案，欢迎试用 CloudTTY 🎉！

## 适用场景

CloudTTY 适用于以下几个场景。

1. 如果企业正使用容器云平台来管理 Kubernetes，但由于安全原因，无法随意 SSH 到主机上执行 kubectl 命令，这就需要一种 Cloud Shell 能力。

2. 在浏览器网页上进入运行中的容器(`kubectl exec`)的场景。

3. 在浏览器网页上能够滚动展示容器日志的场景。

CloudTTY 的网页终端使用效果如下：

![screenshot_gif](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.gif)

如果将 CloudTTY 集成到您自己的 UI 里面，最终效果 demo 如下：

![demo_png](https://github.com/cloudtty/cloudtty/raw/main/docs/demo.png)

## 快速入门步骤

CloudTTY 的入门比较简单，请参照以下步骤进行安装和使用。

1. 安装并等待 Pod 运行起来。

   ```shell
   helm repo add cloudtty https://cloudtty.github.io/cloudtty
   helm repo update
   helm install cloudtty-operator --version 0.5.0 cloudtty/cloudtty
   kubectl wait deployment cloudtty-operator-controller-manager --for=condition=Available=True
   ```

2. 创建 CR，启动 cloudtty 的实例，并观察其状态。

   ```shell
   kubectl apply -f https://raw.githubusercontent.com/cloudtty/cloudtty/v0.5.0/config/samples/local_cluster_v1alpha1_cloudshell.yaml
   ```

   更多范例，参见 [config/samples/](./config/samples/)。

3. 观察 CR 状态，获取访问接入点：

   ```shell
   kubectl get cloudshell -w
   ```

   输出类似于：

   ```console
   NAME                 USER   COMMAND  TYPE        URL                 PHASE   AGE
   cloudshell-sample    root   bash     NodePort    192.168.4.1:30167   Ready   31s
   cloudshell-sample2   root   bash     NodePort    192.168.4.1:30385   Ready   9s
   ```

   当 cloudshell 对象状态变为 `Ready`，并且 `URL` 字段出现之后，就可以通过该字段的访问方式，在浏览器打开：

   ![screenshot_png](https://github.com/cloudtty/cloudtty/raw/main/docs/snapshot.png)

### 怎么构建自定义镜像

大多数用户除了使用基本的 `kubectl` 工具来管理集群外，还需要更多丰富的工具来管理集群。可以基于 cloudshell 的基础镜像来自定义，下面是一个添加 `karmadactl` 工具的一个案例：

* 修改 [Dockerfile.example](https://github.com/cloudtty/cloudtty/blob/main/docker/Dockerfile.example) 文件。

  ```shell
  FROM ghcr.io/cloudtty/cloudshell:v0.6.0

  RUN curl -fsSLO https://github.com/karmada-io/karmada/releases/download/v1.2.0/kubectl-karmada-linux-amd64.tgz \
      && tar -zxf kubectl-karmada-linux-amd64.tgz \
      && chmod +x kubectl-karmada \
      && mv kubectl-karmada /usr/local/bin/kubectl-karmada \
      && which kubectl-karmada

  ENTRYPOINT ttyd
  ```

* 重新构建带有 karmadactl 工具的新镜像：

  ```shell
  docker build -t <IMAGE> . -f docker/Dockerfile-webtty
  ```

### 使用自定义的 cloudshell 镜像

我们有两种方式来设置 cloudshell 的自定义镜像：

1. 直接通过 cloudshell CR 字段 `spec.image` 来设置。

   ```yaml
   apiVersion: cloudshell.cloudtty.io/v1alpha1
   kind: CloudShell
   metadata:
     name: cloudshell-sample
   spec:
     secretRef:
       name: "my-kubeconfig"
     image: ghcr.io/cloudtty/customize_cloudshell:latest
   ```

2. 在安装 CloudTTY 时可以设置 `JobTemplate` 镜像参数来运行自己的 cloudshell 的镜像。

   ```shell
   helm install cloudtty-operator --version 0.5.0 cloudtty/cloudtty --set jobTemplate.image.registry=</REGISTRY> --set jobTemplate.image.repository=</REPOSITORY> --set jobTemplate.image.tag=</TAG>
   ```

> 如果你已经安装了 CloudTTY，还可以修改 `JobTemplate` 的 ConfigMap 来设置 cloudshell 的镜像。

### 使用基于麒麟(Kylin)操作系统的 cloudshell 镜像

对于需要国产化（麒麟）操作系统运行环境的使用场景，仓库提供了基于 [麒麟服务器操作系统 V10 SP3](https://www.kylinos.cn/) 构建的官方镜像，对应 Dockerfile 为 [docker/cloudshell/Dockerfile.kylin](docker/cloudshell/Dockerfile.kylin)。该镜像保留了完整的 CloudShell 工具链（helm、kubectl、yq、ttyd、rz/sz、git、SSH、jq、bash 补全），并且 `rz`/`sz` 仅依赖麒麟自带的 glibc，不会引入 Alpine 库。

- 镜像由 [Build & Release Image](.github/workflows/build-image-release.yaml) 流水线自动构建并发布，支持 `linux/amd64` 和 `linux/arm64` 两种架构：
  - 推送 `main` 分支 → `ghcr.io/cloudtty/cloudshell-kylin:latest`
  - 打版本标签（`v*`）→ `ghcr.io/cloudtty/cloudshell-kylin:<tag>`
  - 涉及麒麟 Dockerfile 的 PR → 只构建校验，不推送
  - 也可以通过 **Actions → Build & Release Kylin Image → Run workflow** 手动触发
- 本地构建：

  ```shell
  make docker-build-kylin KYLIN_TTY_IMG=<IMAGE>
  # 或者
  docker build -t <IMAGE> . -f docker/cloudshell/Dockerfile.kylin
  ```

- 使用方式和自定义镜像一致，例如通过 `spec.image` 指定：

  ```yaml
  apiVersion: cloudshell.cloudtty.io/v1alpha1
  kind: CloudShell
  metadata:
    name: cloudshell-sample
  spec:
    image: ghcr.io/cloudtty/cloudshell-kylin:latest
  ```

## 进阶用法

### 进阶 1：用 CloudTTY 访问其他集群

如果是本地集群，可以不提供 kubeconfig，CloudTTY 会创建具有 `cluster-admin` 角色权限的 `serviceaccount`。
在容器的内部，`kubectl` 会自动发现 `ca` 证书和 token。如果有安全方面的考虑，您也可以自己提供 kubeconfig 来控制不同用户的权限。

如果是远端集群，CloudTTY 可以执行 kubectl 命令行工具。若访问集群，需要指定 kubeconfig。
用户需自己提供 kubeconfig 并储存在 Secret 中，并且在 `cloudshell` 的 CR 中，通过 `spec.secretRef.name` 指定 Secret 的名称。
CloudTTY 会自动挂载到容器中，请确保服务器地址与集群网络连接顺畅。

设置 kubeconfig 的步骤：准备 `kube.conf`，放入 Secret 中，并确保密钥/证书是 base64 而不是本地文件。

```shell
kubectl create secret generic my-kubeconfig --from-file=kube.config
```

### 进阶 2：用 CloudTTY 访问集群上的 node 主机

可以在 cloudshell 的基础镜像中集成 [kubectl-node-shell](https://github.com/kvaps/kubectl-node-shell) 插件，使用该插件可以通过 `kubectl` 的命令登录到集群中任意节点上。该命令将会在节点上启动一个具有特权 Pod。如果对安全性要求非常高，请谨慎使用此功能。

* 修改 [Dockerfile.example](https://github.com/cloudtty/cloudtty/blob/main/docker/Dockerfile.example) 文件。

  ```shell
  FROM ghcr.io/cloudtty/cloudshell:v0.6.0

  RUN curl -fsSLO https://github.com/kvaps/kubectl-node-shell/raw/master/kubectl-node_shell \
      && chmod +x ./kubectl-node_shell \
      && mv ./kubectl-node_shell /usr/local/bin/kubectl-node_shell \
      && which kubectl-node_shell

  ENTRYPOINT ttyd
  ```

* 重新构建带有 node-shell 工具的新镜像：

  ```shell
  docker build -t <IMAGE> . -f docker/Dockerfile.example
  ```

下面是一个使用的实例：

```yaml
apiVersion: cloudshell.cloudtty.io/v1alpha1
kind: CloudShell
metadata:
  name: cloudshell-node-shell
spec:
  configmapName: "<KUBECONFIg>"
  commandAction: "kubectl node-shell <NODE_NAME>"
```

更多的示例可以参考 [kubectl-node-shell](https://github.com/kvaps/kubectl-node-shell)。

> 集群中如果已经存在 `PodSecurity` 和 `PSP` 等安全性策略，可能会影响该功能的使用。

### 进阶 3：修改服务暴露方式

CloudTTY 提供了以下 4 种服务暴露模式以满足不同的使用场景：

* `ClusterIP`：在集群中创建 ClusterIP 类型的 [Service](https://kubernetes.io/zh-cn/docs/concepts/services-networking/service/) 资源。
  适用于第三方集成 CloudTTY 服务，用户可以选择更加灵活的方式来暴露自己的服务。

* `NodePort`：这是默认的模式，也是最简单的暴露服务模式。
  在集群中创建 NodePort 类型的 Service 资源，通过节点 IP 和对应的端口号访问 CloudTTY 服务。

* `Ingress`：在集群中创建 ClusterIP 类型的 Service 资源，并创建 Ingress 资源，通过路由规则负载到 Service 上。
  适合在集群中使用 [Ingress Controller](https://kubernetes.io/zh-cn/docs/concepts/services-networking/ingress-controllers/) 进行流量负载的情况。

* `VirtualService (istio)`：在集群中创建 ClusterIP 类型的 Service 资源，并创建 VirtaulService 资源。
  适合在集群中使用 [Istio](https://github.com/istio/istio) 进行流量负载的情况。

### featureGate

AllowSecretStoreKubeconfig：使用 secret 的方式存储 kubeconfig 文件，如果开启此 featureGate，该字段 `spec.configmapName` 将会失效，使用 `spec.secretRef.name` 来设置 kubeconfig, 目前处于 alpha 阶段，默认是关闭。

#### 如何开启 featrueGate

1. 如果使用 yaml 方式部署 CloudTTY，在 operator 的启动参数中添加 `--feature-gates=AllowSecretStoreKubeconfig=true`.
2. 如果使用 helm 部署的情况，安装指定参数 `--set image.featureGates.AllowSecretStoreKubeconfig=true`.

### 工作原理

CloudTTY 的工作原理如下：

1. Operator 会在对应的命名空间（Namespace）下创建同名的 `job` 和 `service`。
   如果使用 Ingress 或者 VirtualService 模式，还会创建对应的路由信息。

2. 当 Pod 运行状态为 `Ready` 之后，就将访问点写入 cloudshell 的 status 里。

3. 当 [Job](https://kubernetes.io/zh-cn/docs/concepts/workloads/controllers/job/) 在 TTL 之后或者因其他原因结束之后，
   一旦 Job 状态变为 `Completed`，cloudshell 的状态也会变为 `Completed`。
   我们可以设置当 cloudshell 的状态为 `Completed` 时，同时删除相关联的资源。

4. 当 cloudshell 被删除时，会自动删除对应的 Job 和 Service (通过 `ownerReference`)。
   如果使用 Ingress 或者 VirtualService 模式时，还会删除对应的路由信息。

## 开发者模式

CloudTTY 还提供了开发者模式。

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
     secretRef:
       name: "my-kubeconfig"
     runAsUser: "root"
     commandAction: "kubectl -n kube-system logs -f kube-apiserver-cn-stack"
     once: false
   ```

### 开发指南

基于 kubebuilder 框架开发；以 ttyd 为基础构建镜像。

1. 初始化框架，初始化 kubebuilder 项目。

   ```shell
   kubebuilder init --domain daocloud.io --repo daocloud.io/cloudshell
   kubebuilder create api --group cloudshell --version v1alpha1 --kind CloudShell
   ```

2. 生成 manifest。

   ```shell
   make manifests
   ```

3. 调试（使用默认的 kube.conf）。

   ```shell
   make install # 在目标集群安装 CRD
   make run     # 启动 Operator 的代码
   ```

4. 构建镜像。

   ```shell
   make docker-build
   make docker-push
   ```

5. 生成 Operator 部署的 yaml。使用 kustomize 渲染 CRD yaml。

   ```shell
   make generate-yaml
   ```

6. 构建 helm 包。

   ```shell
   make build-chart
   ```

> 开发注意事项：
>
> go get 的 gotty 暂不能用，需要下载：
>
> ```
> wget https://github.com/yudai/gotty/releases/download/v1.0.1/gotty_linux_amd64.tar.gz
> ```

### Docker 镜像和用 Docker 做简化实验

镜像在 master 分支的 docker/目录下，具体操作步骤如下：

1. 创建 ConfigMap 或 kube.conf

   ```shell
   kubectl create configmap my-kubeconfig --from-file=/root/.kube/config
   ```

2. 根据不同场景操作：

   * 日常 kubectl 的 console。

     ```bash
     bash run.sh
     ```

   * 实时查看 Event。

     ```bash
     bash run.sh "kubectl get event -A -w"
     ```

   * 实时查看 Pod 日志。

     ```bash
     NS=caas-system
     POD=caas-api-6d67bfd9b7-fpvdm
     bash run.sh "kubectl -n $NS logs -f $POD"
     ```

## 特别鸣谢

CloudTTY 这个项目的很多技术实现基于 [ttyd](https://github.com/tsl0922/ttyd)，非常感谢 `tsl0922`、`yudai` 和社区开发者们的努力.

CloudTTY 前端 UI 及其镜像内所用的二进制文件均源于 ttyd 社区。

## 交流和探讨

如果您有任何疑问，请联系我们：

* [Slack 频道](https://cloud-native.slack.com/archives/C03LA6AUF7V)
* 微信交流群：请联系 `calvin0327`(wen.chen@daocloud.io) 加入交流群

非常欢迎大家[提出 issue](https://github.com/cloudtty/cloudtty/issues) 和[发起 PR](https://github.com/cloudtty/cloudtty/pulls)。🎉🎉🎉

## 下一步

CloudTTY 还将提供更多的功能，此处列出一些已经排上日程的开发计划。

1. 通过 RBAC 生成的 `/var/run/secret` 进行权限控制
2. 代码还未做边界处理（如 NodePort 准备工作）
3. 为了安全，Job 应该在单独的 Namespace 跑，而不是在 CR 中用同一个 Namespace
4. 需要检查 Pod 的 Running 和 endpoint 的 Ready，才能置 CR 为 Ready
5. 目前 TTL 只反映到 shell 的 timeout，没有反映到 Job 的 yaml 里
6. Job 的创建模板目前是 hardcode 方式，应该提供更灵活的方式修改 Job 的模板

## 贡献者

<a href="https://github.com/cloudtty/cloudtty/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=cloudtty/cloudtty" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

<p align="center">
<img src="https://landscape.cncf.io/images/left-logo.svg" width="300"/>&nbsp;&nbsp;<img src="https://landscape.cncf.io/images/right-logo.svg" width="350"/>
<br/><br/>
cloudtty 位列 <a href="https://landscape.cncf.io/?selected=cloud-tty">CNCF 云原生全景图。</a>
</p>
