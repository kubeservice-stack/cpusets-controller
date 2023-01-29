# 基于节点cpu精细化调度：cpusets-controller
[![Docker publish](https://github.com/kubeservice-stack/cpusets-controller/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/kubeservice-stack/cpusets-controller/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubeservice-stack/cpusets-controller)](https://goreportcard.com/report/github.com/kubeservice-stack/cpusets-controller)

**问：在 Kubernetes 中，运行多个集群节点是否存在隐藏成本？**
答：是的，因为并非 Kubernetes 节点中的所有 `CPU` 和 `Memory` 都可用于运行 `Pod`。

在一个 `Kubernetes 节点`中，`CPU` 和 `Memory` 分为：

- 操作系统
- Kubelet、CNI、CRI、CSI（+ 系统守护进程）
- Pods
- 驱逐门槛(Hard Eviction threshold)

`Kubelet` 原生的`CPU Manager` 对于每一次`Pod`调度 `binding` 到 `Node` 时, 都会将计算 Node CPU是否够用

Node预留CPU资源 reservedCPUs = KubeReservedCPUs + SystemReservedCPUs + HardEvictionThresholds ， 并向上取整，最终最为reserved cpus

![](https://www.kubeservice.cn/img/k8s-kubelet/cpu-manager.png)

虽然，对预留资源做了`限制`，但对于具体的`核没有做到绑核`运行。如果需要对Pod进行`绑核、亲核`部署 就再精细化实现了

![](https://www.kubeservice.cn/img/k8s-kubelet/cpu-manager-reserved.png)


## 场景

如果需要对 `kubernetes` 中使用 `CPU 管理器` 进行如下更加精细化管理：
![](https://www.kubeservice.cn/img/k8s-kubelet/cpu-pooler.png)

- 解决在一个容器中可以同时使用独占cpu和共享cpu
- 支持conatiner级别 core亲和 和 绑核
- 兼容历史申请资源，优雅cgroup cpu驱逐
- 独占CPU支持 绑核

解决 传统应用`上云`，特定业务`绑核`运行 和 核心业务分级CPU亲和（`CPU Arrinity`）部署

## 方案

### 方案一：kubelet 的 cm上直接扩展
直接在 `Kubelet`中，添加`CPUSets`对象中，添加`DeviceID` 和具体拓扑使用，并记录到`CheckPoints`文件中。

优势：性能优，并且与`Kubelet预分配`资源做到协同
缺点：
- 1）更改原生代码，非云原生；
- 2）`多Kubernetes`管理和升级复杂；
- 3）`Kubernetes社区`难落地，抽象和迭代困难

### 方案二：做云原生调度插件，替换/选用 `kubelet CPU Manager` 逻辑

`Cpusets Controller` 提供一种 `Kubernetes` 的设备插件，将 `CPU` 内核作为可 `Device` 的 `Kubernetes` 调度程序.

支持三种类型的 CPU 管理：
- CPU独占
- CPU共享
- CPU默认（兼容默认cpu方式）

![](https://www.kubeservice.cn/img/k8s-kubelet/cpuset.png)


包含 `3` 个核心组件：

- `device plugin`: `Kubernetes` 标准`Device`插件，将 CPU 池作为可调度资源无缝集成到 Kubernetes
- `controller`: `Kubernetes` 标准 `Informer`，确保属于不同 `CPU Pooler` 的容器始终在物理上相互隔离管理和设置
- `webhook`: 准入 Webhook验证, 校验 CPU 池特定的用户请求是否合法


![](https://www.kubeservice.cn/img/k8s-kubelet/controller-cpuset.png)

`Device Plugin` 的工作是通过现有的 `Device Plugin API` 将 `cpu分配` 作为`可消耗资源`注册给 `Kubelet`. `Device Plugin` 将 `CPU` 作为包含`物理CPU ID`列表的环境变量传递给容器。默认情况下，应用程序可以根据给定的 `CPU` 列表设置其进程 `CPU` 亲和力，或者可以将其留给标准的 `Linux Completely Fair Scheduler`。对于应用程序未实现设置其进程的 `CPU` 亲和力的功能的边缘情况，`CPU Pool`提供了代表应用程序设置它的机制。通过将应用程序进程信息配置到其 `Pod 规范`的注释字段来启用此选择加入功能。

`Webhook`: 以根据启动二进制文件（安装、环境变量等）的需要来改变 Pod 的规范。

`Controller` 子组件通过 `Linux cpusets` 实现容器的完全物理分离。通过 `Informer` 不断地监视 `Kubernetes` 的 `Pod API`，并在创建 Pod 或更改其状态（例如重新启动等）时触发。 `shared` 在共享的情况下，或者默认情况下容器没有明确要求任何池化资源。`Controller` 然后将计算出的集合提供给容器的 `cgroupfs` 文件系统 (`cpuset.cpus`) 。

优势：
- 1)对CPU进一步池化，实现部分绑核能力；
- 2)对现有的部署不影响
- 3)特定绑核使用，使大颗粒高性能服务可上云；

缺点：
- 1)default 与 shared共享问题
- 2)Kubelet CPU Manager 与 CPUSet Controller协同问题
