# 这是一个cloudshell的opeartor

简体中文 | [英文](https://github.com/cloudtty/cloudtty/blob/main/README.md)

# 特别鸣谢
这个项目的很多技术实现都是基于`https://github.com/tsl0922/ttyd`, 非常感谢 `tsl0922` `yudai`和社区.
前端UI也是从 `ttyd` 项目衍生出来的，另外镜像内所使用的`ttyd`二进制也是来源于这个项目。




# 为什么需要cloudtty ?

像ttyd等项目已经非常成熟了，可以提供浏览器之上的终端的能力。
但是在kubernetes的场景下，我们需要能有更云原生的能力拓展，比如ttyd在容器内运行，能够通过nodePort等方式访问，能够用CRD的方式创建多个实例。

# 适用场景

1. 很多企业使用容器云平台来管理Kubernetes, 但是由于安全原因，无法随意SSH到主机上执行kubectl，就需要一种Cloud Shell的能力。
2. 在浏览器网页上能够进入运行中的容器(`kubectl exec`)的场景
3. 在浏览器网页上能够滚动展示容器日志的场景




### 用法：

0.前置条件
 -  创建kubeconf的configmap（这样能在pod里使用kubectl, 如果目标集群跟operator是同一个集群，未来会优化步骤）
    - （第一步）`kubectl create configmap my-kubeconfig --from-file=/root/.kube/config`
    - （第二步）然后编辑这个configmap, 修改endpoint的地址，从IP改为servicename, 如`server: https://kubernetes.default.svc.cluster.local:443`


1. 运行operator和安装CRD

  a) 直接用户: 从Helm Chart 部署(推荐)

	```
	helm repo add daocloud  https://release.daocloud.io/chartrepo/cloudshell
	helm install --version 0.0.2 daocloud/cloudshell --generate-name
	```

  b) 开发者: 编译执行 （建议普通用户使用上述Helm安装）

      b.1 ) 安装CRD

        - （选择1）从YAML： 
	   ```make generate-yaml
             然后apply 生成的yaml```

        - （选择2）从代码：克隆代码之后 `make install`

      b.2 ) 运行Operator
        `make run`


2. 创建cloudshell的CR

-（选择1）使用范例
   ```
   kubectl apply -f config/samples/cloudshell_v1alpha1_cloudshell.yaml
   kubectl get cloudshells  -w
   ```

更多范例，参加`config/samples/`


- (选择2) 自定义CR， 比如开启窗口后自动打印某个容器的日志：
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


3. operator会在对应的NS下创建同名的 `job` 和`service`（nodePort）

4. 当pod运行ready之后，就将nodeport的访问点写入CR的status里,效果如下

```
kubectl get cloudshell
NAME                 USER   COMMAND   URL            PHASE   AGE
cloudshell-sample    root   bash      NodeIP:30167   Ready   31s
cloudshell-sample2   root   bash      NodeIP:30385   Ready   9s
```

5. 当job在TTL或者其他原因结束之后，一旦job变为Completed，CR的状态也会变成`Completed`

6. 当CRD被删除时，会自动删除对应的job和service(通过`ownerReference`)


ToDo：

- （1）通过RBAC生成的/var/run/secret，进行权限控制
- （2）前端添加sz/xz的按钮, 修改https://github.com/tsl0922/ttyd/html里的前端代码，添加按钮。或者第二个页面类似iframe嵌入？`ttyd --index=new-index.html`
- （3）要加入ingress和Gateway API, 以及Istio的VirtualService的不同暴露方式的选择 🔥
- （4）代码中边界处理（如nodeport准备好）还没有处理
- （5）暂时还是ttyd运行once（一个客户端断开即退出），以方便调试
- （6）为了安全，job应该在单独的NS跑，而不是在CR同NS
-  (7) 需要检查pod的Running和endpoint的Ready，才能置位CRD为Ready
-  (8)目前TTL只反映到shell的timeout，没有反映到job的yaml里












# 开发指南

基于kubebuilder框架开发
基于ttyd为基础，构建镜像

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

5. 生成operator部署的yaml（暂未helm化）
```
#use kustomize to render CRD yaml
make generate-yaml
```

#开发注意：

#go get的gotty不能用...要下载
wget https://github.com/yudai/gotty/releases/download/v1.0.1/gotty_linux_amd64.tar.gz



===================
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

c) 实时查看pod日志
```
NS=caas-system
POD=caas-api-6d67bfd9b7-fpvdm
bash run.sh "kubectl -n $NS logs -f $POD"
```


