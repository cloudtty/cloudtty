# 这是一个cloudshell的opeartor


### 用法：

0.前置条件
 - a) 安装CRD
        - （选择1）从YAML： ```make generate-yaml
             然后apply 生成的yaml```
        - （选择2）从代码：克隆代码之后 `make install`
 - b) 创建kubeconf的configmap（这样能在pod里使用kubectl）
    - （第一步）`kubectl create configmap my-kubeconfig --from-file=/root/.kube/config`
    - （第二步）然后编辑这个configmap, 修改endpoint的地址，从IP改为servicename, 如`server: https://kubernetes.default.svc.cluster.local:443`


1.用户创建cloudshell的CR
- 范例在 `config/samples/webtty_v1alpha1_cloudshell.yaml`
 -   ` kubectl apply -f config/samples/webtty_v1alpha1_cloudshell.yaml  && kubectl get cloudshells  -w`


- 自己写一个CR， 比如开启窗口后自动打印某个容器的日志：
    ```
    apiVersion: webtty.webtty.daocloud.io/v1alpha1
    kind: CloudShell
    metadata:
      name: cloudshell-sample
    spec:
      configmapName: "kube-config"
      commandAction: "kubectl -n kube-system logs -f kube-apiserver-cn-stack"
    ```


2.operator会在对应的NS下创建同名的 `job` 和`service`（nodePort）

4.当pod运行ready之后，就将nodeport的访问点写入CR的status里,效果如下
```
kubectl get cloudshell
NAME                 USER   COMMAND   URL            PHASE   AGE
cloudshell-sample    root   bash      NodeIP:30167   Ready   31s
cloudshell-sample2   root   bash      NodeIP:30385   Ready   9s
```

5.当job在TTL或者其他原因结束之后，一旦job变为Completed，CR的状态也会变成`Completed`

5.当CRD被删除时，会自动删除对应的job和service(通过`ownerReference`)


ToDo：

- （1）还未把CR里的user注入kube.conf（动态生成），进行权限控制
- （2）前端添加sz/xz的按钮, 修改https://github.com/tsl0922/ttyd/html里的前端代码，添加按钮。或者第二个页面类似iframe嵌入？`ttyd --index=new-index.html`
- （3）代码中边界处理（如nodeport准备好）还没有处理
- （4）要加入ingress和Gateway API的不同暴露方式的选择
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


